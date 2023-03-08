CREATE TABLE raw_packet (
  id SERIAL PRIMARY KEY,
  payload BYTEA,
  sender INET,
);
