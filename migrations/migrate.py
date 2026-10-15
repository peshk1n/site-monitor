import argparse
import hashlib
import os
from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import parse_qsl, unquote, urlsplit


def connection_environment(database_url=None, base=None):
    environment = (os.environ if base is None else base).copy()
    environment.setdefault('PGCONNECT_TIMEOUT', '10')
    database_url = database_url if database_url is not None else environment.get('DATABASE_URL')
    if not database_url:
        return environment
    try:
        parsed = urlsplit(database_url)
        if parsed.scheme not in ('postgres', 'postgresql') or parsed.fragment:
            raise ValueError
        parameters = {
            'host': unquote(parsed.hostname) if parsed.hostname else None,
            'port': str(parsed.port) if parsed.port is not None else None,
            'user': unquote(parsed.username) if parsed.username is not None else None,
            'password': unquote(parsed.password) if parsed.password is not None else None,
            'dbname': unquote(parsed.path[1:]) if parsed.path else None,
        }
        parameters.update(parse_qsl(parsed.query, keep_blank_values=True))
    except ValueError:
        raise ValueError('DATABASE_URL must be a valid PostgreSQL connection URI') from None
    names = {
        'host': 'PGHOST', 'hostaddr': 'PGHOSTADDR', 'port': 'PGPORT', 'user': 'PGUSER',
        'password': 'PGPASSWORD', 'dbname': 'PGDATABASE', 'sslmode': 'PGSSLMODE',
        'sslcert': 'PGSSLCERT', 'sslkey': 'PGSSLKEY', 'sslrootcert': 'PGSSLROOTCERT',
        'sslcrl': 'PGSSLCRL', 'sslcrldir': 'PGSSLCRLDIR', 'connect_timeout': 'PGCONNECT_TIMEOUT',
        'application_name': 'PGAPPNAME', 'options': 'PGOPTIONS',
        'target_session_attrs': 'PGTARGETSESSIONATTRS', 'channel_binding': 'PGCHANNELBINDING',
    }
    if parameters.keys() - names.keys():
        raise ValueError('Unsupported DATABASE_URL parameter; use standard PG* variables instead')
    for key, value in parameters.items():
        if value is not None:
            environment[names[key]] = value
    return environment


def literal(value):
    return "'" + str(value).replace("'", "''") + "'"


def load_migrations(directory):
    migrations = []
    for path in sorted((directory / 'up').glob('*.up.sql')):
        match = re.fullmatch(r'(\d+)_([a-z0-9_]+)\.up\.sql', path.name)
        if match is None:
            raise ValueError(f'Invalid migration filename: {path.name}')
        version, name = int(match[1]), match[2]
        down = directory / 'down' / path.name.replace('.up.sql', '.down.sql')
        if not down.is_file():
            raise ValueError(f'Missing rollback: {down.name}')
        migrations.append({
            'version': version,
            'name': name,
            'up': path.read_text(),
            'down': down.read_text(),
            'up_checksum': hashlib.sha256(path.read_bytes()).hexdigest(),
            'down_checksum': hashlib.sha256(down.read_bytes()).hexdigest(),
        })
    migrations.sort(key=lambda migration: migration['version'])
    if [m['version'] for m in migrations] != list(range(1, len(migrations) + 1)) or not migrations:
        raise ValueError('Migration versions must be contiguous, starting at 1')
    if len(list((directory / 'down').glob('*.down.sql'))) != len(migrations):
        raise ValueError('Unexpected rollback files')
    return migrations


def build_script(migrations, command, target):
    lines = [
        'BEGIN;',
        "SET LOCAL client_min_messages = warning;",
        "SET LOCAL lock_timeout = '10s';",
        "SELECT pg_advisory_xact_lock(hashtextextended("
        "current_database() || ':' || current_schema() || ':site-monitor-migrations', 0));",
        'CREATE TABLE IF NOT EXISTS schema_migrations ('
        'version INTEGER PRIMARY KEY, name TEXT NOT NULL, up_checksum TEXT NOT NULL, '
        'down_checksum TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);',
        'CREATE TEMP TABLE migration_catalog ('
        'version INTEGER PRIMARY KEY, name TEXT, up_checksum TEXT, down_checksum TEXT) ON COMMIT DROP;',
    ]
    for migration in migrations:
        values = ', '.join(literal(migration[key]) for key in
                           ('version', 'name', 'up_checksum', 'down_checksum'))
        lines.append(f'INSERT INTO migration_catalog VALUES ({values});')
    lines.append('''DO $validation$
BEGIN
    IF EXISTS (
        SELECT 1 FROM schema_migrations s LEFT JOIN migration_catalog c USING (version)
        WHERE c.version IS NULL OR s.name <> c.name
            OR s.up_checksum <> c.up_checksum OR s.down_checksum <> c.down_checksum
    ) THEN
        RAISE EXCEPTION 'Applied migration files differ from their recorded checksums';
    END IF;
    IF (SELECT count(*) FROM schema_migrations) <>
       (SELECT coalesce(max(version), 0) FROM schema_migrations) THEN
        RAISE EXCEPTION 'Migration history is not contiguous';
    END IF;
END;
$validation$;''')
    if command == 'baseline':
        lines.append('''DO $baseline$
BEGIN
    IF EXISTS (SELECT 1 FROM schema_migrations) THEN
        RAISE EXCEPTION 'Baseline requires an empty migration history';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_name <> 'schema_migrations'
    ) THEN
        RAISE EXCEPTION 'Baseline requires an existing, manually verified schema';
    END IF;
END;
$baseline$;''')
        lines.append(f'INSERT INTO schema_migrations (version, name, up_checksum, down_checksum) '
                     f'SELECT version, name, up_checksum, down_checksum FROM migration_catalog '
                     f'WHERE version <= {target};')
    else:
        lines.append('''DO $untracked$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM schema_migrations) AND EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_name <> 'schema_migrations'
    ) THEN
        RAISE EXCEPTION 'Untracked schema: verify it, then use baseline --to VERSION';
    END IF;
END;
$untracked$;''')
        if command == 'up':
            lines.append(f'''DO $direction$
BEGIN
    IF (SELECT coalesce(max(version), 0) FROM schema_migrations) > {target} THEN
        RAISE EXCEPTION 'Target precedes the current version; use down explicitly';
    END IF;
END;
$direction$;''')
            for migration in migrations[:target]:
                version = migration['version']
                lines.extend([
                    f'SELECT NOT EXISTS (SELECT 1 FROM schema_migrations WHERE version = {version}) '
                    'AS apply_migration \\gset',
                    '\\if :apply_migration',
                    f"\\echo Applying {version:03d}_{migration['name']}",
                    migration['up'],
                    'INSERT INTO schema_migrations (version, name, up_checksum, down_checksum) '
                    f'SELECT version, name, up_checksum, down_checksum FROM migration_catalog '
                    f'WHERE version = {version};',
                    '\\endif',
                ])
        elif command == 'down':
            lines.append(f'''DO $direction$
BEGIN
    IF (SELECT coalesce(max(version), 0) FROM schema_migrations) < {target} THEN
        RAISE EXCEPTION 'Target exceeds the current version; use up explicitly';
    END IF;
END;
$direction$;''')
            for migration in reversed(migrations[target:]):
                version = migration['version']
                lines.extend([
                    f'SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = {version}) '
                    'AS revert_migration \\gset',
                    '\\if :revert_migration',
                    f"\\echo Reverting {version:03d}_{migration['name']}",
                    migration['down'],
                    f'DELETE FROM schema_migrations WHERE version = {version};',
                    '\\endif',
                ])
    lines.extend([
        'SELECT c.version, c.name, CASE WHEN s.version IS NULL THEN \'pending\' ELSE \'applied\' END '
        'AS status FROM migration_catalog c LEFT JOIN schema_migrations s USING (version) ORDER BY c.version;',
        'ROLLBACK;' if command == 'status' else 'COMMIT;',
    ])
    return '\n'.join(lines) + '\n'


def main():
    parser = argparse.ArgumentParser(
        description='Apply PostgreSQL migrations using psql and a transactional version journal.',
        epilog=(
            'Connection: standard PG* variables, or a PostgreSQL URI in DATABASE_URL. '
            'No .env file is loaded. Docker init scripts run only on an empty data directory; '
            'for an existing schema verify its exact version before using baseline --to N. '
            'Baseline records checksums without inspecting or changing the existing application tables. '
            'Legacy timestamps are interpreted as UTC. '
            'Rollback discards new tables/fields and collapses result outcomes to is_up; '
            'it cannot restore information lost by a schema conversion.'
        ),
    )
    parser.add_argument('command', choices=('up', 'down', 'status', 'baseline'))
    parser.add_argument('--to', type=int, help='Target version; required for down and baseline')
    parser.add_argument('--allow-data-loss', action='store_true', help='Required for rollback')
    parser.add_argument('--psql', default='psql', help='Path to the psql executable')
    parser.add_argument('--directory', type=Path, default=Path(__file__).resolve().parent)
    args = parser.parse_args()
    try:
        migrations = load_migrations(args.directory)
        target = args.to if args.to is not None else len(migrations)
        if not 0 <= target <= len(migrations):
            parser.error('Target must be between 0 and the latest migration version')
        if args.command in ('down', 'baseline') and args.to is None:
            parser.error('--to is required for down and baseline')
        if args.command == 'baseline' and target == 0:
            parser.error('Baseline requires a positive target version')
        if args.command == 'down' and not args.allow_data_loss:
            parser.error('Rollback requires --allow-data-loss')
        environment = connection_environment()
        result = subprocess.run(
            [args.psql, '-X', '-v', 'ON_ERROR_STOP=1', '-f', '-'],
            input=build_script(migrations, args.command, target), text=True, env=environment,
        )
        return result.returncode
    except (OSError, ValueError) as error:
        print(f'Migration error: {error}', file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
