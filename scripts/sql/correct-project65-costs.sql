-- MySQL 8.0+. Execute in the application's database, in a new session.
-- Only biz_cooperations.quote_amount / updated_at for project_id = 65 can change.
-- Does not alter budgets, metrics, resources, links, or any other project.
-- Requires SELECT, UPDATE, CREATE, INSERT, CREATE ROUTINE, ALTER ROUTINE,
-- EXECUTE and CREATE TEMPORARY TABLES privileges.
-- CALL ... (0) previews and rolls back; CALL ... (1) validates and commits.
-- Do not use mysql --force. Stop project 65 edits/imports during execution.
-- The backup table intentionally survives for audit and recovery.
-- DDL stays outside the transaction (MySQL implicitly commits normal DDL).

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
    correct_total_cents BIGINT NOT NULL
  ) ENGINE=InnoDB;
  -- Identity hashes and package totals derived from the supplied workbook.
  -- No names, contact information or published URLs are stored here.
  INSERT INTO tmp_p65_source VALUES
  (1,'08554d02509f101d8ee4dd9e4c808a88d1e2d89e028a4136fbda5c766e485e32',7,22700,22700),
  (2,'0ecd783505707ed7ca71d093a48d36459fa7867b17999e4aa902fba7efdda998',2,9000,9000),
  (3,'6499a9c43a0d7a2d0d7a7aa864fa007b08060f3f279fe6cc7b4bbd487706c158',2,31800,31800),
  (4,'b2c2760a7ebc7834ba4b34227ab9aaf98188177d53a6690cfe81f34ba61544a7',6,27200,27200),
  (5,'82280bb7b30905f97322961a56e3c119150a246cb47293607c23385110c73d52',7,136300,136300),
  (6,'79a2bf65cc048979a14c03f449f46a59f24d8a74e3d151595468b9971c7e146d',1,20000,20000),
  (7,'9b338a5f3b80e1af36ddd530779616d800d795dcba948022e999e7f07df9763b',3,366900,366900),
  (8,'9c10d897732a312407dd7ff168aa428e063f99121526a49945fb92a414bb0b8d',7,27200,27200),
  (9,'a48c848b68a3b188a24144fef6991d426d84268aac93e7d6d42346793948c02a',4,40900,40900),
  (10,'a04ec138b0ac26b709a0475238a8d4a61a61187cce35000e80d95e66b1bfe125',4,18100,18100),
  (11,'4efd8af546fa6bd5db5668a9d864fe96ae8cbb6ffca00dd443dfa5494cf88baa',7,0,0),
  (12,'92cd357548fb2bd831f965df6fcab0e11debb410f2f123842d3cd6dd5d8f0455',5,13600,13600),
  (13,'ec5bac21835b96545bc77b9a4d2ccca79d18778a16bb1d35b2997313517b1718',5,18100,18100),
  (14,'304eb4dd01f2fd3f93e78c9085318d56133b3999eea47e9fb80ab205fc676fcb',1,0,0);

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
  IF v_count <> 61 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Expected exactly 61 cooperation rows; no change made';
  END IF;
  SELECT COUNT(*) INTO v_count FROM tmp_p65_snapshot
   WHERE currency IS NULL OR UPPER(TRIM(currency)) <> 'USD'
      OR resource_type IS NULL OR LOWER(resource_type) REGEXP '媒体|media'
      OR import_batch_id IS NULL OR import_batch_id NOT LIKE 'IMP%';
  IF v_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Unexpected currency, resource type or non-imported row';
  END IF;
  SELECT COUNT(*) INTO v_count FROM tmp_p65_snapshot s
    LEFT JOIN tmp_p65_source p ON p.name_sha256 = s.name_sha256
   WHERE p.group_id IS NULL;
  IF v_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Collaborator identity differs from workbook; no change made';
  END IF;
  SELECT COUNT(*) INTO v_count FROM (
    SELECT p.group_id FROM tmp_p65_source p
    LEFT JOIN tmp_p65_snapshot s ON s.name_sha256 = p.name_sha256
    GROUP BY p.group_id, p.expected_rows
    HAVING COUNT(s.cooperation_id) <> p.expected_rows
  ) mismatches;
  IF v_count <> 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Cooperation group counts differ from workbook';
  END IF;

  INSERT INTO tmp_p65_targets
  SELECT ranked.cooperation_id, p.group_id, ranked.original_amount,
         (FLOOR(p.correct_total_cents / p.expected_rows)
          + IF(ranked.position_in_group <= MOD(p.correct_total_cents,p.expected_rows),1,0)) / 100
  FROM (
    SELECT cooperation_id, name_sha256, original_amount,
           ROW_NUMBER() OVER (PARTITION BY name_sha256 ORDER BY cooperation_id) AS position_in_group
      FROM tmp_p65_snapshot
  ) ranked JOIN tmp_p65_source p ON p.name_sha256 = ranked.name_sha256;
  SELECT SUM(corrected_amount) INTO v_after FROM tmp_p65_targets;
  IF v_after <> 7318.00 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Planned total must be exactly 7318.00';
  END IF;

  -- A second run is a no-op only when the original backup and every row agree.
  SELECT COUNT(*) INTO v_count FROM ops_project65_cost_backup_20261030;
  IF v_count <> 0 THEN
    IF v_count <> 61 OR v_before <> 7318.00 THEN
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
     WHERE s.original_amount * 100 <> p.original_unit_cents;
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
       WHERE c.project_id = 65 AND c.quote_amount = t.original_amount;
      SET v_changed = ROW_COUNT();

      SELECT COUNT(*), COALESCE(SUM(quote_amount),0)
        INTO v_count, v_after FROM biz_cooperations WHERE project_id = 65;
      IF v_count <> 61 OR v_after <> 7318.00 THEN
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

-- Independent final verification (expected: 61 rows, 7318.00 total_usd):
-- SELECT COUNT(*) AS cooperation_rows, SUM(quote_amount) AS total_usd
-- FROM biz_cooperations WHERE project_id = 65;
