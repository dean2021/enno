// Package enno provides a provider-neutral agent runtime for Go applications.
//
// The core type is Agent. Basic callers can use Agent.Run to get the final text
// answer, while advanced callers can use RunDetailed, RunSession, RunStream,
// events, policies, and hooks to inspect or control execution.
package enno
