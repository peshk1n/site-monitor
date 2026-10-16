import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
import uuid


MIGRATIONS = Path(__file__).resolve().parents[1]
RUNNER = MIGRATIONS / 'migrate.py'
LATEST_VERSION = max(int(path.name.split('_')[0]) for path in (MIGRATIONS / 'up').glob('*.up.sql'))
sys.path.insert(0, str(MIGRATIONS))
from migrate import connection_environment


class MigrationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        database = os.environ.get('MIGRATION_TEST_DATABASE_URL')
        if not database:
            raise unittest.SkipTest('Set MIGRATION_TEST_DATABASE_URL to an isolated PostgreSQL 16 database')
        cls.psql = os.environ.get('PSQL', 'psql')
        cls.base_environment = connection_environment(database)
        cls.base_environment['DATABASE_URL'] = ''
        cls.base_environment['PGCONNECT_TIMEOUT'] = '5'
        cls.base_environment.pop('PGOPTIONS', None)

    def setUp(self):
        self.schema = 'migration_test_' + uuid.uuid4().hex
        self.environment = self.base_environment.copy()
        self.sql(f'CREATE SCHEMA {self.schema}', environment=self.base_environment)
        self.addCleanup(self.sql, f'DROP SCHEMA {self.schema} CASCADE', environment=self.base_environment)
        self.environment['PGOPTIONS'] = f'-c search_path={self.schema} -c timezone=UTC'

    def sql(self, statement, expected_error=None, environment=None):
        result = subprocess.run(
            [self.psql, '-X', '-qAt', '-v', 'ON_ERROR_STOP=1', '-v', 'VERBOSITY=verbose', '-c', statement],
            text=True, capture_output=True, env=environment or self.environment,
        )
        if expected_error:
            self.assertNotEqual(result.returncode, 0, statement)
            self.assertIn(expected_error, result.stderr)
        else:
            self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout.strip()

    def migrate(self, command='up', target=None, directory=None, error=None):
        argv = [sys.executable, str(RUNNER), command, '--psql', self.psql]
        if target is not None:
            argv.extend(['--to', str(target)])
        if command == 'down':
            argv.append('--allow-data-loss')
        if directory:
            argv.extend(['--directory', str(directory)])
        result = subprocess.run(argv, text=True, capture_output=True, env=self.environment)
        if error:
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(error, result.stderr)
        else:
            self.assertEqual(result.returncode, 0, result.stderr)
        return result

    def fixtures(self):
        self.migrate()
        self.project = int(self.sql("SELECT id FROM projects WHERE slug = 'default'"))
        self.other_project = int(self.sql(
            "INSERT INTO projects (slug, name) VALUES ('other', 'Other') RETURNING id"))
        self.user = int(self.sql(
            "INSERT INTO users (login, display_name) VALUES ('owner', 'Owner') RETURNING id"))
        self.monitor = self.make_monitor(self.project, 'main')
        self.other_monitor = self.make_monitor(self.other_project, 'other')

    def make_monitor(self, project, name):
        return int(self.sql(
            f"INSERT INTO monitors (project_id, name, url) VALUES ({project}, '{name}', "
            f"'https://{name}.example') RETURNING id"))

    def make_result(self, monitor):
        return int(self.sql(
            'INSERT INTO check_results (monitor_id, config_version, scheduled_at, started_at, '
            'finished_at, observed_at, response_ms, status_code, outcome) '
            f"VALUES ({monitor}, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, "
            "CURRENT_TIMESTAMP, 25, 200, 'success') RETURNING id"))

    def make_incident(self, monitor, project):
        return int(self.sql(
            'INSERT INTO incidents (monitor_id, project_id, severity, first_observed_at) '
            f"VALUES ({monitor}, {project}, 'critical', CURRENT_TIMESTAMP) RETURNING id"))

    def make_event(self, incident, monitor, project):
        return self.sql(
            'INSERT INTO monitor_events (incident_id, monitor_id, project_id, kind, severity, '
            'config_version, deduplication_key) '
            f"VALUES ({incident}, {monitor}, {project}, 'incident_opened', 'critical', 1, "
            f"'{uuid.uuid4()}') RETURNING id")

    def legacy_shape(self):
        return self.sql(
            "SELECT table_name, column_name, data_type, is_nullable, coalesce(column_default, '') "
            "FROM information_schema.columns WHERE table_schema = current_schema() "
            "AND table_name IN ('monitors', 'checks') ORDER BY table_name, column_name")

    def test_fresh_upgrade_idempotency_and_full_rollback(self):
        self.migrate(target=2)
        original_shape = self.legacy_shape()
        self.migrate()
        self.migrate()
        self.assertEqual(self.sql('SELECT count(*) FROM schema_migrations'), str(LATEST_VERSION))
        self.migrate('status')
        self.migrate('down', 2)
        self.assertEqual(self.legacy_shape(), original_shape)
        self.migrate()
        self.migrate('down', 0)
        self.assertEqual(self.sql(
            "SELECT count(*) FROM information_schema.tables WHERE table_schema = current_schema() "
            "AND table_name <> 'schema_migrations'"), '0')
        self.migrate()

    def test_legacy_records_are_preserved_without_fabricated_observations(self):
        self.migrate(target=2)
        self.sql("INSERT INTO monitors (url, interval, timeout, created_at) "
                 "VALUES ('https://legacy.example', 60, 10, '2026-01-01 10:00:00'); "
                 "INSERT INTO checks (monitor_id, status_code, response_ms, is_up, error, checked_at) "
                 "VALUES (1, 0, 0, false, 'timeout', '2026-01-02 10:00:00'), "
                 "(1, 200, 25, true, '', '2026-01-03 10:00:00')")
        self.migrate()
        self.assertEqual(self.sql(
            'SELECT origin, outcome, status_code IS NULL, config_version IS NULL, '
            "started_at IS NULL, observed_at = '2026-01-02 10:00:00+00'::TIMESTAMPTZ "
            'FROM check_results WHERE id = 1'), 'legacy|failure|t|t|t|t')
        self.assertEqual(self.sql('SELECT health_status, candidate_count FROM monitor_runtime'), 'unknown|0')
        self.assertEqual(self.sql('SELECT count(*) FROM incidents'), '0')
        self.sql('DELETE FROM monitors WHERE id = 1', expected_error='23503')
        self.migrate('down', 2)
        self.assertEqual(self.sql('SELECT url, interval, timeout FROM monitors'), 'https://legacy.example|60|10')
        self.assertEqual(self.sql('SELECT is_up, status_code, error FROM checks WHERE id = 1'), 'f|0|timeout')
        self.assertEqual(self.sql('SELECT is_up, status_code, response_ms, error IS NOT NULL FROM checks WHERE id = 2'),
                         't|200|25|t')

    def test_monitor_configuration_version_and_validation(self):
        self.fixtures()
        self.sql(f"UPDATE monitors SET name = 'Renamed', config_version = 99 WHERE id = {self.monitor}")
        self.assertEqual(self.sql(f'SELECT config_version FROM monitors WHERE id = {self.monitor}'), '1')
        self.sql(f"UPDATE monitors SET url = 'https://changed.example' WHERE id = {self.monitor}")
        self.assertEqual(self.sql(f'SELECT config_version FROM monitors WHERE id = {self.monitor}'), '2')
        for assignment in [
            'timeout_seconds = 0', 'interval_seconds = -1', 'failure_threshold = 0',
            "url = 'file:///etc/passwd'", "expected_status_codes = '{}'", 'expected_status_codes = ARRAY[99]',
            'expected_status_codes = ARRAY[200, NULL]::SMALLINT[]', "body_match_type = 'regexp'",
            'warning_response_ms = 100000', 'max_body_bytes = 0', 'max_redirects = -1',
        ]:
            with self.subTest(assignment=assignment):
                self.sql(f'UPDATE monitors SET {assignment} WHERE id = {self.monitor}', expected_error='23514')

    def test_telegram_identity_is_unique_and_supports_large_ids(self):
        self.fixtures()
        telegram_id = 2 ** 40
        self.sql(f'UPDATE users SET telegram_user_id = {telegram_id} WHERE id = {self.user}')
        self.sql("UPDATE users SET display_name = 'Renamed' WHERE id = " + str(self.user))
        self.assertEqual(self.sql(f'SELECT telegram_user_id FROM users WHERE id = {self.user}'), str(telegram_id))
        self.sql("INSERT INTO users (login, display_name, telegram_user_id) "
                 f"VALUES ('duplicate', 'Duplicate', {telegram_id})", expected_error='23505')
        for invalid in (0, -1):
            with self.subTest(telegram_id=invalid):
                self.sql(f'UPDATE users SET telegram_user_id = {invalid} WHERE id = {self.user}',
                         expected_error='23514')
        self.sql(f'UPDATE users SET archived_at = CURRENT_TIMESTAMP WHERE id = {self.user}')
        self.sql("INSERT INTO users (login, display_name, telegram_user_id) "
                 f"VALUES ('replacement', 'Replacement', {telegram_id})", expected_error='23505')
        self.sql("INSERT INTO users (login, display_name) VALUES ('legacy-a', 'A'), ('legacy-b', 'B')")
        self.assertEqual(self.sql('SELECT count(*) FROM users WHERE telegram_user_id IS NULL'), '2')

    def test_state_candidate_requires_a_complete_observed_series(self):
        self.fixtures()
        self.sql('INSERT INTO monitor_runtime (monitor_id, state_config_version) '
                 f'VALUES ({self.monitor}, 1)')
        self.assertEqual(self.sql('SELECT candidate_status IS NULL, candidate_count, '
                                 'candidate_since IS NULL, evaluation_started_at IS NOT NULL '
                                 'FROM monitor_runtime'), 't|0|t|t')
        self.sql("UPDATE monitor_runtime SET candidate_status = 'down', candidate_count = 1, "
                 'candidate_since = CURRENT_TIMESTAMP', expected_error='23514')
        self.sql("UPDATE monitor_runtime SET health_status = 'up', last_observed_at = CURRENT_TIMESTAMP, "
                 "fresh_until = CURRENT_TIMESTAMP + INTERVAL '130 seconds', candidate_status = 'down', "
                 'candidate_count = 2, candidate_since = CURRENT_TIMESTAMP')
        for assignment in [
            "candidate_status = 'unknown'", "candidate_status = 'up'", 'candidate_status = NULL',
            'candidate_count = 0', 'candidate_count = -1', 'candidate_count = 101',
            'candidate_count = NULL', 'candidate_since = NULL',
            "candidate_since = last_observed_at + INTERVAL '1 second'",
            'last_observed_at = NULL, fresh_until = NULL', "health_status = 'down'",
        ]:
            with self.subTest(assignment=assignment):
                expected = '23502' if assignment == 'candidate_count = NULL' else '23514'
                self.sql(f'UPDATE monitor_runtime SET {assignment}', expected_error=expected)
        self.sql("UPDATE monitor_runtime SET health_status = 'down', candidate_status = NULL, "
                 'candidate_count = 0, candidate_since = NULL')
        self.sql("UPDATE monitor_runtime SET candidate_status = 'degraded', candidate_count = 1, "
                 'candidate_since = last_observed_at')
        self.assertEqual(self.sql('SELECT health_status, candidate_status, candidate_count FROM monitor_runtime'),
                         'down|degraded|1')

    def test_mini_app_upgrade_and_rollback_preserve_confirmed_state(self):
        self.migrate(target=10)
        project = int(self.sql("SELECT id FROM projects WHERE slug = 'default'"))
        monitor = self.make_monitor(project, 'existing')
        result = self.make_result(monitor)
        incident = self.make_incident(monitor, project)
        self.make_event(incident, monitor, project)
        self.sql("INSERT INTO users (login, display_name) VALUES ('existing', 'Existing')")
        self.sql('INSERT INTO monitor_runtime (monitor_id, state_config_version, health_status, '
                 'consecutive_successes, last_check_id, last_observed_at, fresh_until, active_run_id, '
                 'lease_owner, lease_expires_at, running_config_version) '
                 f"VALUES ({monitor}, 1, 'down', 1, {result}, CURRENT_TIMESTAMP, "
                 "CURRENT_TIMESTAMP + INTERVAL '130 seconds', gen_random_uuid(), 'worker', "
                 "CURRENT_TIMESTAMP + INTERVAL '30 seconds', 1)")
        state_query = ('SELECT monitor_id, health_status, state_config_version, last_check_id, last_observed_at, '
                       'fresh_until, active_run_id, lease_owner, lease_expires_at, running_config_version, '
                       'state_changed_at FROM monitor_runtime')
        original_state = self.sql(state_query)
        self.migrate()
        self.assertEqual(self.sql(state_query), original_state)
        self.assertEqual(self.sql('SELECT candidate_status IS NULL, candidate_count FROM monitor_runtime'), 't|0')
        self.assertEqual(self.sql('SELECT telegram_user_id IS NULL FROM users'), 't')
        self.sql('UPDATE users SET telegram_user_id = 1099511627776')
        self.sql("UPDATE monitor_runtime SET candidate_status = 'up', candidate_count = 1, "
                 'candidate_since = last_observed_at')
        self.migrate('down', 10)
        self.assertEqual(self.sql(state_query), original_state)
        self.assertEqual(self.sql('SELECT consecutive_failures, consecutive_successes FROM monitor_runtime'), '0|0')
        self.assertEqual(self.sql('SELECT login FROM users'), 'existing')
        self.assertEqual(self.sql('SELECT count(*) FROM incidents WHERE resolved_at IS NULL'), '1')
        self.assertEqual(self.sql('SELECT count(*) FROM monitor_events'), '1')
        self.assertEqual(self.sql('SELECT count(*) FROM check_results'), '1')
        self.migrate()
        self.assertEqual(self.sql(state_query), original_state)
        self.assertEqual(self.sql('SELECT candidate_status IS NULL, candidate_count FROM monitor_runtime'), 't|0')
        self.assertEqual(self.sql('SELECT telegram_user_id IS NULL FROM users'), 't')

    def test_access_requests_have_independent_lifecycles(self):
        self.fixtures()
        request = (f'INSERT INTO access_requests (project_id, user_id, requested_role) '
                   f"VALUES ({self.project}, {self.user}, 'viewer')")
        self.sql(request)
        self.sql(request, expected_error='23505')
        self.sql("UPDATE access_requests SET status = 'approved'", expected_error='23514')
        self.sql("UPDATE access_requests SET status = 'approved', decided_at = CURRENT_TIMESTAMP, "
                 f'decided_by = {self.user}')
        self.assertEqual(self.sql('SELECT count(*) FROM project_members'), '0')
        self.sql(f"INSERT INTO project_members (project_id, user_id, role) "
                 f"VALUES ({self.project}, {self.user}, 'viewer')")
        self.sql(f"INSERT INTO project_members (project_id, user_id, role) "
                 f"VALUES ({self.project}, {self.user}, 'owner')", expected_error='23505')
        self.sql(request)

    def test_results_require_observations_and_unique_run_ids(self):
        self.fixtures()
        self.sql(f"INSERT INTO check_results (monitor_id, outcome) VALUES ({self.monitor}, 'success')",
                 expected_error='23514')
        result = self.make_result(self.monitor)
        self.sql('INSERT INTO check_results (monitor_id, run_id, origin, outcome) '
                 f"SELECT monitor_id, run_id, 'legacy', outcome FROM check_results WHERE id = {result}",
                 expected_error='23505')
        for assignment in [
            'config_version = 0', 'response_ms = -1', 'status_code = 600',
            "finished_at = started_at - INTERVAL '1 second'", 'accepted = false',
        ]:
            with self.subTest(assignment=assignment):
                self.sql(f'UPDATE check_results SET {assignment} WHERE id = {result}', expected_error='23514')
        self.sql(f"UPDATE check_results SET accepted = false, discard_reason = 'outdated configuration' WHERE id = {result}")

    def test_incident_isolation_and_result_retention(self):
        self.fixtures()
        result = self.make_result(self.monitor)
        other_result = self.make_result(self.other_monitor)
        self.sql(f'INSERT INTO monitor_runtime (monitor_id, state_config_version, last_check_id) '
                 f'VALUES ({self.monitor}, 1, {result})')
        self.sql(f'UPDATE monitor_runtime SET last_check_id = {other_result} '
                 f'WHERE monitor_id = {self.monitor}', expected_error='23503')
        incident = self.make_incident(self.monitor, self.project)
        self.sql('INSERT INTO incidents (monitor_id, project_id, severity, first_observed_at) '
                 f"VALUES ({self.monitor}, {self.project}, 'critical', CURRENT_TIMESTAMP)", expected_error='23505')
        self.sql('INSERT INTO incidents (monitor_id, project_id, severity, first_observed_at) '
                 f"VALUES ({self.other_monitor}, {self.project}, 'critical', CURRENT_TIMESTAMP)", expected_error='23503')
        self.make_event(incident, self.monitor, self.project)
        self.sql(f'DELETE FROM check_results WHERE id = {result}')
        self.assertEqual(self.sql('SELECT last_check_id IS NULL FROM monitor_runtime'), 't')
        self.assertEqual(self.sql('SELECT count(*) FROM incidents'), '1')
        self.sql(f'UPDATE monitors SET archived_at = CURRENT_TIMESTAMP WHERE id = {self.monitor}')
        self.assertEqual(self.sql('SELECT count(*) FROM monitor_events'), '1')
        self.sql(f'DELETE FROM monitors WHERE id = {self.monitor}', expected_error='23503')
        self.sql(f"UPDATE incidents SET resolved_at = CURRENT_TIMESTAMP, resolution = 'recovered' WHERE id = {incident}")
        self.make_incident(self.monitor, self.project)

    def test_notification_routing_and_delivery_constraints(self):
        self.fixtures()
        channel = int(self.sql('INSERT INTO notification_channels (project_id, name, kind, destination) '
                               f"VALUES ({self.project}, 'Chat', 'telegram', '-100123') RETURNING id"))
        foreign_channel = int(self.sql('INSERT INTO notification_channels (project_id, name, kind, destination) '
                                       f"VALUES ({self.other_project}, 'Other', 'webhook', 'https://hooks.example') RETURNING id"))
        rule = int(self.sql('INSERT INTO notification_rules (project_id, name, scope) '
                            f"VALUES ({self.project}, 'Failures', 'monitors') RETURNING id"))
        self.sql(f'INSERT INTO notification_rule_monitors (rule_id, project_id, monitor_id) '
                 f'VALUES ({rule}, {self.project}, {self.other_monitor})', expected_error='23503')
        self.sql(f'INSERT INTO notification_rule_monitors (rule_id, project_id, monitor_id) '
                 f'VALUES ({rule}, {self.project}, {self.monitor})')
        self.sql(f'INSERT INTO notification_rule_channels (rule_id, project_id, channel_id) '
                 f'VALUES ({rule}, {self.project}, {foreign_channel})', expected_error='23503')
        self.sql(f'INSERT INTO notification_rule_channels (rule_id, project_id, channel_id) '
                 f'VALUES ({rule}, {self.project}, {channel})')
        incident = self.make_incident(self.monitor, self.project)
        event = self.make_event(incident, self.monitor, self.project)
        delivery = ('INSERT INTO notification_deliveries (project_id, event_id, channel_id, channel_kind, destination) '
                    f"VALUES ({self.project}, '{event}', {channel}, 'telegram', '-100123')")
        self.sql(delivery)
        self.sql(delivery, expected_error='23505')
        self.sql("UPDATE notification_deliveries SET status = 'sent'", expected_error='23514')
        self.sql("UPDATE notification_deliveries SET status = 'processing'", expected_error='23514')
        self.sql("UPDATE notification_deliveries SET status = 'processing', lease_token = gen_random_uuid(), "
                 "lease_owner = 'worker', lease_expires_at = CURRENT_TIMESTAMP + INTERVAL '30 seconds'")
        self.sql("UPDATE notification_deliveries SET status = 'pending', attempts = 1, "
                 "last_attempt_at = CURRENT_TIMESTAMP, last_error = 'temporary error', "
                 "next_attempt_at = CURRENT_TIMESTAMP + INTERVAL '1 minute', "
                 'lease_token = NULL, lease_owner = NULL, lease_expires_at = NULL')
        self.sql("UPDATE notification_deliveries SET status = 'sent', attempts = 2, "
                 'last_attempt_at = CURRENT_TIMESTAMP, sent_at = CURRENT_TIMESTAMP, last_error = NULL')
        self.assertEqual(self.sql('SELECT status, attempts FROM notification_deliveries'), 'sent|2')

    def test_silence_scope_and_window(self):
        self.fixtures()
        silence = int(self.sql('INSERT INTO silences (kind, scope, starts_at, ends_at, reason, created_by) '
                               f"VALUES ('maintenance', 'projects', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 hour', "
                               f"'Deploy', {self.user}) RETURNING id"))
        self.sql(f'INSERT INTO silence_projects (silence_id, project_id) VALUES ({silence}, {self.project})')
        self.sql(f'INSERT INTO silence_projects (silence_id, project_id) VALUES ({silence}, {self.other_project})')
        self.sql(f'INSERT INTO silence_monitors (silence_id, project_id, monitor_id) '
                 f'VALUES ({silence}, {self.project}, {self.monitor})', expected_error='23503')
        self.sql('UPDATE silences SET ends_at = starts_at', expected_error='23514')
        self.sql("UPDATE silences SET scope = 'all'", expected_error='23503')
        self.sql('UPDATE silences SET cancelled_at = CURRENT_TIMESTAMP', expected_error='23514')

    def test_concurrent_workers_claim_distinct_monitors(self):
        self.fixtures()
        self.sql('INSERT INTO monitor_runtime (monitor_id, state_config_version) SELECT id, config_version FROM monitors')
        query = '''BEGIN;
WITH due AS (
    SELECT monitor_id FROM monitor_runtime
    WHERE next_run_at <= CURRENT_TIMESTAMP AND active_run_id IS NULL
    ORDER BY next_run_at, monitor_id FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE monitor_runtime r SET active_run_id = gen_random_uuid(), lease_owner = 'worker',
    lease_expires_at = CURRENT_TIMESTAMP + INTERVAL '30 seconds', running_config_version = 1
FROM due WHERE r.monitor_id = due.monitor_id RETURNING r.monitor_id;
SELECT pg_sleep(0.2);
COMMIT;'''
        processes = [subprocess.Popen([self.psql, '-X', '-qAt', '-v', 'ON_ERROR_STOP=1', '-c', query],
                                      text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                      env=self.environment) for _ in range(2)]
        claimed = []
        for process in processes:
            output, error = process.communicate(timeout=10)
            self.assertEqual(process.returncode, 0, error)
            claimed.append(output.strip())
        self.assertEqual(len(set(claimed)), 2)
        self.assertEqual(self.sql('SELECT count(*) FROM monitor_runtime WHERE active_run_id IS NOT NULL'), '2')
        self.sql('UPDATE monitor_runtime SET active_run_id = NULL', expected_error='23514')

    def test_untracked_schema_requires_explicit_baseline(self):
        self.sql((MIGRATIONS / 'up/001_create_monitors.up.sql').read_text())
        self.sql((MIGRATIONS / 'up/002_create_checks.up.sql').read_text())
        self.migrate(error='Untracked schema')
        self.migrate('baseline', 2)
        self.migrate()
        self.migrate('baseline', 10, error='empty migration history')

    def test_initialization_scripts_can_be_baselined(self):
        for path in sorted((MIGRATIONS / 'up').glob('*.up.sql')):
            self.sql(path.read_text())
        self.migrate('baseline', LATEST_VERSION)
        self.migrate()
        self.assertEqual(self.sql('SELECT count(*) FROM projects'), '1')
        self.assertEqual(self.sql('SELECT count(*) FROM schema_migrations'), str(LATEST_VERSION))

    def test_checksums_and_failed_migration_are_transactional(self):
        self.migrate()
        with tempfile.TemporaryDirectory(prefix='site-monitor-migrations-') as temporary:
            directory = Path(temporary)
            for direction in ('up', 'down'):
                shutil.copytree(MIGRATIONS / direction, directory / direction)
            path = directory / 'up/003_projects_and_access.up.sql'
            path.write_text(path.read_text() + '\n')
            self.migrate(directory=directory, error='checksums')
            shutil.copyfile(MIGRATIONS / 'up/003_projects_and_access.up.sql', path)
            (directory / f'up/{LATEST_VERSION + 1:03d}_broken.up.sql').write_text('CREATE TABLE failed_step (id INTEGER); SELECT 1 / 0;')
            (directory / f'down/{LATEST_VERSION + 1:03d}_broken.down.sql').write_text('DROP TABLE failed_step;')
            self.migrate(directory=directory, error='division by zero')
            self.assertEqual(self.sql("SELECT to_regclass('failed_step') IS NULL"), 't')
            self.assertEqual(self.sql('SELECT max(version) FROM schema_migrations'), str(LATEST_VERSION))
            self.assertEqual(self.sql('SELECT count(*) FROM projects'), '1')


if __name__ == '__main__':
    unittest.main()
