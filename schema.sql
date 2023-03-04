CREATE TABLE raw_packet (
  id SERIAL PRIMARY KEY,
  payload BYTEA,
  sender INET,
  received INT DEFAULT 0
);


CREATE OR REPLACE FUNCTION notify_raw_packet_ready()
  RETURNS TRIGGER AS $$
DECLARE
    row RECORD;
BEGIN
  row = NEW;
  PERFORM pg_notify('raw_packet_ready', '' || row.id);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE TRIGGER "raw_notify"
AFTER INSERT ON raw_packet
FOR EACH ROW EXECUTE PROCEDURE notify_raw_packet_ready();
