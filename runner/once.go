package runner

import (
	"context"

	"github.com/dean2021/enno"
)

func Once(ctx context.Context, agent *enno.Agent, prompt string) (string, error) {
	return agent.Run(ctx, prompt)
}
