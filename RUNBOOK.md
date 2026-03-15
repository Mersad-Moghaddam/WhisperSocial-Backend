# RUNBOOK

## Grant admin role

```sql
UPDATE users SET role='admin' WHERE email='admin@example.com';
```

## User moderation

Use admin API `PATCH /admin/users/{id}` with one of:
- `active`
- `deactivated`
- `restricted`

Operational impact:
- deactivated users cannot log in and are filtered from timeline/feed queries.
- restricted users are blocked from creating posts.

## Post moderation

Use `DELETE /admin/posts/{id}`. This soft-deletes the post and removes its ID from follower timeline Redis ZSET keys.
