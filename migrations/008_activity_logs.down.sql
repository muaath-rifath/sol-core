SELECT remove_retention_policy('room_activity_logs', if_exists => TRUE);
DROP TABLE IF EXISTS room_activity_logs CASCADE;
