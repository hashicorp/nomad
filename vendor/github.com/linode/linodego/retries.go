package linodego

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
)

const retryAfterHeaderName = "Retry-After"

// type RetryConditional func(r *resty.Response) (shouldRetry bool)
type RetryConditional resty.RetryConditionFunc

// Configures resty to
// lock until enough time has passed to retry the request as determined by the Retry-After response header.
// If the Retry-After header is not set, we fall back to value of SetPollDelay.
func configureRetries(c *Client) {
	c.resty.
		SetRetryCount(1000).
		AddRetryCondition(checkRetryConditionals(c)).
		SetRetryAfter(respectRetryAfter)
}

func checkRetryConditionals(c *Client) func(*resty.Response, error) bool {
	return func(r *resty.Response, err error) bool {
		for _, retryConditional := range c.retryConditionals {
			retry := retryConditional(r, err)
			if retry {
				log.Printf("[INFO] Received error %s - Retrying", r.Error())
				return true
			}
		}
		return false
	}
}

// SetLinodeBusyRetry configures resty to retry specifically on "Linode busy." errors
// The retry wait time is configured in SetPollDelay
func linodeBusyRetryCondition(r *resty.Response, _ error) bool {
	apiError, ok := r.Error().(*APIError)
	linodeBusy := ok && apiError.Error() == "Linode busy."
	retry := r.StatusCode() == http.StatusBadRequest && linodeBusy
	return retry
}

func tooManyRequestsRetryCondition(r *resty.Response, _ error) bool {
	return r.StatusCode() == http.StatusTooManyRequests
}

func serviceUnavailableRetryCondition(r *resty.Response, _ error) bool {
	return r.StatusCode() == http.StatusServiceUnavailable
}

func respectRetryAfter(client *resty.Client, resp *resty.Response) (time.Duration, error) {
	retryAfterStr := resp.Header().Get(retryAfterHeaderName)
	if retryAfterStr == "" {
		return 0, nil
	}

	retryAfter, err := strconv.Atoi(retryAfterStr)
	if err != nil {
		return 0, err
	}

	duration := time.Duration(retryAfter) * time.Second
	log.Printf("[INFO] Respecting Retry-After Header of %d (%s) (max %s)", retryAfter, duration, client.RetryMaxWaitTime)
	return duration, nil
}
