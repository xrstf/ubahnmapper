CREATE TABLE IF NOT EXISTS "ubahnmapper" (
  "time"      TIMESTAMP(3)      NOT NULL,
  "run_id"    VARCHAR(25)       NOT NULL,
  "pressure"  DOUBLE PRECISION  NOT NULL,
  "comment"   VARCHAR(100)      NULL,
  PRIMARY KEY ("time", "run_id")
);
