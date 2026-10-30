-- MySQL 8.0+. Execute in the application's database, in a new session.
-- Only biz_cooperations.quote_amount / updated_at for project_id = 65 can change.
-- Does not alter budgets, metrics, resources, links, or any other project.
-- Requires SELECT, UPDATE, CREATE, INSERT, CREATE ROUTINE, ALTER ROUTINE,
-- EXECUTE and CREATE TEMPORARY TABLES privileges.
-- CALL ... (0) previews and rolls back; CALL ... (1) validates and commits.
-- Do not use mysql --force. Stop project 65 edits/imports during execution.
-- The backup table intentionally survives for audit and recovery.
-- DDL stays outside the transaction (MySQL implicitly commits normal DDL).
-- Revision 2: verified server distribution has 62 rows, including one extra
-- zero-cost row in group 4. Preserve that row; split its package across 6 paid rows.

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS ops_project65_cost_backup_20261030 (
  cooperation_id BIGINT NOT NULL PRIMARY KEY,
  project_id BIGINT NOT NULL,
  original_amount DECIMAL(14,2) NOT NULL,
  corrected_amount DECIMAL(14,2) NOT NULL,
  original_updated_at DATETIME NULL,
  corrected_at DATETIME NOT NULL,
  CONSTRAINT chk_p65_cost_backup CHECK (project_id = 65)
) ENGINE=InnoDB;

DROP PROCEDURE IF EXISTS ops_correct_project65_costs_20261030;
DELIMITER $$
CREATE PROCEDURE ops_correct_project65_costs_20261030(IN p_apply INT)
SQL SECURITY INVOKER
proc: BEGIN
  DECLARE v_count INT DEFAULT 0;
  DECLARE v_project BIGINT DEFAULT NULL;
  DECLARE v_before DECIMAL(14,2) DEFAULT 0;
  DECLARE v_after DECIMAL(14,2) DEFAULT 0;
  DECLARE v_changed INT DEFAULT 0;
  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    ROLLBACK;
    DROP TEMPORARY TABLE IF EXISTS tmp_p65_targets;
    DROP TEMPORARY TABLE IF EXISTS tmp_p65_snapshot;
    DROP TEMPORARY TABLE IF EXISTS tmp_p65_source;
    RESIGNAL;
  END;

  IF p_apply IS NULL OR p_apply NOT IN (0, 1) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Use 0 for preview or 1 for apply';
  END IF;

  SELECT COUNT(*) INTO v_count FROM information_schema.tables
   WHERE table_schema = DATABASE()
     AND table_name IN ('biz_projects','biz_cooperations','biz_resources',
                        'ops_project65_cost_backup_20261030')
     AND engine = 'InnoDB';
  IF v_count <> 4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Required tables missing or not InnoDB';
  END IF;
  SELECT COUNT(*) INTO v_count FROM information_schema.triggers
   WHERE trigger_schema = DATABASE()
     AND event_object_table IN ('biz_cooperations','ops_project65_cost_backup_20261030');
  IF v_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Triggers found: review side effects before correcting';
  END IF;

  CREATE TEMPORARY TABLE tmp_p65_source (
    group_id INT NOT NULL PRIMARY KEY,
    name_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL UNIQUE,
    expected_rows INT NOT NULL,
    original_unit_cents BIGINT NOT NULL,
    correct_total_cents BIGINT NOT NULL,
    preserved_zero_rows INT NOT NULL
  ) ENGINE=InnoDB;
  -- Current identity hashes from the operator's server query; package totals
  -- and charged-row counts reconciled with the supplied workbook.
  -- No names, contact information or published URLs are stored here.
  INSERT INTO tmp_p65_source VALUES
  (1,'18f278e3737f6479d10fd8969c61bad2dca59000ce3e7f1010c6a3e8b152919f',7,22700,22700,0),
  (2,'3453a611ab1bceb41fd7f8655bc9a299cb6b0372b20cc7add8be905819639c25',2,9000,9000,0),
  (3,'dba6e5d387943bd2321a0d71fa07f98ef53db4f093991174fd46cbb08901f204',2,31800,31800,0),
  (4,'f6183c0523e03deb841f435136b7936b7aa52443a5cddf4aeaa47c0b098b215b',6,27200,27200,1),
  (5,'82280bb7b30905f97322961a56e3c119150a246cb47293607c23385110c73d52',7,136300,136300,0),
  (6,'47c4c82f1436dc04c2a5ecf75982960e365424e90857c68d2442bcadeadb6e56',1,20000,20000,0),
  (7,'58e55487057e1462f9c575f02accc68246c9d6e569ba46cada984d952be64a7e',3,366900,366900,0),
  (8,'86b9787e1cd10220017eb10481c245a954c70047d4972be4534ce3b2455a9fe8',7,27200,27200,0),
  (9,'88a918a2f9b52ec211ef5ca3ed1e2a8485202dc78007ce7a640641c094d7f590',4,40900,40900,0),
  (10,'ad0fe9d0a30c6b4348e8be7eac06bf7a5993410ec4976bf992fd1370a652dce2',4,18100,18100,0),
  (11,'872b8ba4a6cb8709e2d0c9dee86cd7ab43546b66902d7e487f2deb7e564b78ee',7,0,0,0),
  (12,'c7641edb7363f4bf44a9134768b4def8de4a49f0d9afbd314307a0c654ec3fa8',5,13600,13600,0),
  (13,'a54422c17a26de00edd829fc0e40cce7c6ae725e6994c055c558257084d97b91',5,18100,18100,0),
  (14,'f2a93a5aa8e0d31596fffd28940ce2a94ced8c7dcb02efeee4ecb819b9ba3972',1,0,0,0);

  CREATE TEMPORARY TABLE tmp_p65_snapshot (
    cooperation_id BIGINT NOT NULL PRIMARY KEY,
    name_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    original_amount DECIMAL(14,2) NOT NULL,
    original_updated_at DATETIME NULL,
    currency VARCHAR(16),
    resource_type VARCHAR(64),
    import_batch_id VARCHAR(128)
  ) ENGINE=InnoDB;
  CREATE TEMPORARY TABLE tmp_p65_targets (
    cooperation_id BIGINT NOT NULL PRIMARY KEY,
    group_id INT NOT NULL,
    original_amount DECIMAL(14,2) NOT NULL,
    corrected_amount DECIMAL(14,2) NOT NULL
  ) ENGINE=InnoDB;

  SET TRANSACTION ISOLATION LEVEL REPEATABLE READ;
  START TRANSACTION;
  SELECT id INTO v_project FROM biz_projects WHERE id = 65 FOR UPDATE;
  IF v_project IS NULL THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Project 65 does not exist';
  END IF;
  INSERT INTO tmp_p65_snapshot
  SELECT c.id, SHA2(LOWER(TRIM(COALESCE(r.name,''))), 256),
         c.quote_amount, c.updated_at, c.currency, r.resource_type, c.import_batch_id
    FROM biz_cooperations c
    LEFT JOIN biz_resources r ON r.id = c.resource_id
   WHERE c.project_id = 65
   FOR UPDATE;

  SELECT COUNT(*), COALESCE(SUM(original_amount),0)
    INTO v_count, v_before FROM tmp_p65_snapshot;
  IF v_count <> 62 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Expected exactly 62 cooperation rows; no change made';
  END IF;
  SELECT COUNT(*) INTO v_count FROM tmp_p65_snapshot
   WHERE currency IS NULL OR UPPER(TRIM(currency)) <> 'USD'
      OR resource_type IS NULL OR LOWER(resource_type) REGEXP '媒体|media'
      OR (original_amount > 0 AND (import_batch_id IS NULL OR import_batch_id NOT LIKE 'IMP%'));
  IF v_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Unexpected currency, resource type or non-imported row';
  END IF;
  SELECT COUNT(*) INTO v_count FROM tmp_p65_snapshot s
    LEFT JOIN tmp_p65_source p ON p.name_sha256 = s.name_sha256
   WHERE p.group_id IS NULL;
  IF v_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Collaborator identity differs from verified server distribution';
  END IF;
  SELECT COUNT(*) INTO v_count FROM (
    SELECT p.group_id FROM tmp_p65_source p
    LEFT JOIN tmp_p65_snapshot s ON s.name_sha256 = p.name_sha256
    GROUP BY p.group_id, p.expected_rows, p.preserved_zero_rows
    HAVING COUNT(s.cooperation_id) <> p.expected_rows + p.preserved_zero_rows
       OR (p.preserved_zero_rows > 0 AND SUM(s.original_amount = 0) <> p.preserved_zero_rows)
  ) mismatches;
  IF v_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Cooperation group counts differ from verified source distribution';
  END IF;

  INSERT INTO tmp_p65_targets
  SELECT ranked.cooperation_id, ranked.group_id, ranked.original_amount,
         (FLOOR(ranked.correct_total_cents / ranked.expected_rows)
          + IF(ranked.position_in_group <= MOD(ranked.correct_total_cents,ranked.expected_rows),1,0)) / 100
  FROM (
    SELECT s.cooperation_id, s.name_sha256, s.original_amount,
           p.group_id, p.expected_rows, p.correct_total_cents,
           ROW_NUMBER() OVER (PARTITION BY s.name_sha256 ORDER BY s.cooperation_id) AS position_in_group
      FROM tmp_p65_snapshot s JOIN tmp_p65_source p ON p.name_sha256 = s.name_sha256
     WHERE p.preserved_zero_rows = 0 OR s.original_amount > 0
  ) ranked;
  INSERT INTO tmp_p65_targets
  SELECT s.cooperation_id, p.group_id, s.original_amount, 0
    FROM tmp_p65_snapshot s JOIN tmp_p65_source p ON p.name_sha256 = s.name_sha256
   WHERE p.preserved_zero_rows > 0 AND s.original_amount = 0;
  SELECT SUM(corrected_amount) INTO v_after FROM tmp_p65_targets;
  IF v_after <> 7318.00 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Planned total must be exactly 7318.00';
  END IF;

  -- A second run is a no-op only when the original backup and every row agree.
  SELECT COUNT(*) INTO v_count FROM ops_project65_cost_backup_20261030;
  IF v_count <> 0 THEN
    IF v_count <> 62 OR v_before <> 7318.00 THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Existing backup or current total differs; inspect before retry';
    END IF;
    SELECT COUNT(*) INTO v_count FROM tmp_p65_targets t
      LEFT JOIN ops_project65_cost_backup_20261030 b ON b.cooperation_id = t.cooperation_id
     WHERE b.cooperation_id IS NULL OR b.project_id <> 65
        OR b.corrected_amount <> t.corrected_amount
        OR t.original_amount <> t.corrected_amount;
    IF v_count <> 0 THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Current rows differ from previously corrected values';
    END IF;
    COMMIT;
    SELECT 'ALREADY_CORRECTED' AS result, 65 AS project_id, v_before AS total_usd;
  ELSE
    IF v_before <> 30634.00 THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Expected original total 30634.00; no change made';
    END IF;
    SELECT COUNT(*) INTO v_count FROM tmp_p65_snapshot s
      JOIN tmp_p65_source p ON p.name_sha256 = s.name_sha256
     WHERE s.original_amount * 100 <> p.original_unit_cents
       AND NOT (p.preserved_zero_rows > 0 AND s.original_amount = 0);
    IF v_count <> 0 THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Original per-row amounts differ from workbook';
    END IF;

    SELECT 65 AS project_id, COUNT(*) AS cooperation_rows,
           SUM(original_amount) AS before_usd, SUM(corrected_amount) AS after_usd
      FROM tmp_p65_targets;
    SELECT group_id, COUNT(*) AS cooperation_rows,
           SUM(original_amount) AS before_usd, SUM(corrected_amount) AS after_usd
      FROM tmp_p65_targets GROUP BY group_id ORDER BY group_id;

    IF p_apply = 0 THEN
      ROLLBACK;
      SELECT 'PREVIEW_ONLY_NO_CHANGES' AS result;
    ELSE
      INSERT INTO ops_project65_cost_backup_20261030
        (cooperation_id,project_id,original_amount,corrected_amount,original_updated_at,corrected_at)
      SELECT t.cooperation_id,65,t.original_amount,t.corrected_amount,s.original_updated_at,NOW()
        FROM tmp_p65_targets t JOIN tmp_p65_snapshot s USING (cooperation_id);

      UPDATE biz_cooperations c
      JOIN tmp_p65_targets t ON t.cooperation_id = c.id
         SET c.quote_amount = t.corrected_amount
       WHERE c.project_id = 65 AND c.quote_amount = t.original_amount
         AND c.quote_amount <> t.corrected_amount;
      SET v_changed = ROW_COUNT();

      SELECT COUNT(*), COALESCE(SUM(quote_amount),0)
        INTO v_count, v_after FROM biz_cooperations WHERE project_id = 65;
      IF v_count <> 62 OR v_after <> 7318.00 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Post-update check failed; transaction rolled back';
      END IF;
      SELECT COUNT(*) INTO v_count FROM biz_cooperations c
      JOIN tmp_p65_targets t ON t.cooperation_id = c.id
       WHERE c.project_id = 65 AND c.quote_amount <> t.corrected_amount;
      IF v_count <> 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Per-row verification failed; transaction rolled back';
      END IF;
      COMMIT;
      SELECT 'COMMITTED' AS result,65 AS project_id,v_changed AS changed_rows,v_after AS total_usd;
    END IF;
  END IF;

  DROP TEMPORARY TABLE tmp_p65_targets;
  DROP TEMPORARY TABLE tmp_p65_snapshot;
  DROP TEMPORARY TABLE tmp_p65_source;
END$$
DELIMITER ;

-- First invocation previews only. After checking it, run the apply CALL below.
CALL ops_correct_project65_costs_20261030(0);
-- CALL ops_correct_project65_costs_20261030(1);

-- Independent final verification (expected: 62 rows, 7318.00 total_usd):
-- SELECT COUNT(*) AS cooperation_rows, SUM(quote_amount) AS total_usd
-- FROM biz_cooperations WHERE project_id = 65;
