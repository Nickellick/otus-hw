goose \
  -dir migrations \
  postgres \
  "postgres://calendar:calendar@127.0.0.1:10432/calendar?sslmode=disable" \
  up