-- Rooms and automation rules are meaningless without a home — cascade delete them.
ALTER TABLE rooms
    DROP CONSTRAINT rooms_home_id_fkey,
    ADD CONSTRAINT rooms_home_id_fkey
        FOREIGN KEY (home_id) REFERENCES homes(id) ON DELETE CASCADE;

ALTER TABLE automation_rules
    DROP CONSTRAINT automation_rules_home_id_fkey,
    ADD CONSTRAINT automation_rules_home_id_fkey
        FOREIGN KEY (home_id) REFERENCES homes(id) ON DELETE CASCADE;
