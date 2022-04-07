# Backoff
A more natural backoff and retry package.

## Installation
    go get github.com/clarkmcc/backoff

## Example
```go
// The following backoff configuration will start at one second and double
// the duration on every retry for the first five retries, or until we reach
// 10-second intervals. Once we reach 10-second intervals, we keep retrying
// every 10 seconds until we've retried 10 times.
b := backoff.Exponential{
	Duration: time.Second,
	Factor: 2,
	Steps: 5,
	Cap: 10 * time.Second,
	Max: 10
}

for b.Next(ctx) {
	err := failableOperation()
	if err != nil {
		continue
    }
    break
}
```

## Related Packages
* [cenkalti/backoff](https://pkg.go.dev/github.com/cenkalti/backoff/v4#example-Retry) - Requires the use of closures