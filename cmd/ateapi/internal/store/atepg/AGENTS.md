# atepg

This is the database layer.  It is responsible for storing and retrieving data
from the database. It should not have logic that is not related to this
responsibility.

- Never put API validation logic here.  That belongs in the API layers.
- Always include robust tests, in partiular for edge cases, consistency and
  concurrency, error handling and performance.
- All type-specific code should go in the type's own file(s).  For example, all
  Actor related code goes in `actor.go` and all Actor related tests go in
  `actor_test.go`.
