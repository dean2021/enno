// Package enno provides a provider-neutral agent runtime for Go applications.
//
// The core type is Agent. Callers use Agent.Run with an explicit Session and
// receive a structured RunResult. Optional streaming, events, policies, and
// hooks provide additional control and observability.
package enno
