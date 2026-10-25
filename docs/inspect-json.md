# Inspect JSON

`sloth inspect --json URL` writes exactly one compact JSON document to stdout.
The document uses schema identifier `sloth.inspect/v1` and contains a single
`result` object with:

- redacted request metadata and unit-explicit request policy;
- the stable outcome category, status, completion state, and safe error;
- total and phase timing values;
- protocol and observed connection facts;
- bounded response-body byte and completion evidence; and
- redacted redirect hops and final target identity.

Each timing phase is an object. An observed value uses a numeric
`duration_ms`; an unavailable or inapplicable value uses `null` plus an
`unavailable_reason`. An observed zero is therefore encoded as `0`, never
`null`.

```json
{
  "duration_ms": null,
  "unavailable_reason": "connection_reused"
}
```

The JSON document never contains ANSI control sequences. Query values,
credentials, sensitive headers, and request or response body content are not
included. Durations are client-observable and phase values are not guaranteed
to add up to the total.

If request execution begins and ends in a timeout, transport failure,
body-limit failure, or cancellation, Sloth still writes one result document and
then exits with the corresponding nonzero status. Validation and safety-policy
failures occur before execution and therefore emit no result document.
