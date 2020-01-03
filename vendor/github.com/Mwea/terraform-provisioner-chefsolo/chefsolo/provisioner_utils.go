package chefsolo

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform/communicator"
	"github.com/hashicorp/terraform/terraform"
)

func getStringList(v interface{}) []string {
	var result []string

	switch v := v.(type) {
	case nil:
		return result
	case []interface{}:
		for _, vv := range v {
			if vv, ok := vv.(string); ok {
				result = append(result, vv)
			}
		}
		return result
	default:
		panic(fmt.Sprintf("Unsupported type: %T", v))
	}
}

func getCommunicator(ctx context.Context, o terraform.UIOutput, s *terraform.InstanceState) (communicator.Communicator, error) {
	// Get a new communicator
	comm, err := communicator.New(s)
	if err != nil {
		return nil, err
	}

	retryCtx, cancel := context.WithTimeout(ctx, comm.Timeout())
	defer cancel()

	// Wait and retry until we establish the connection
	err = communicator.Retry(retryCtx, func() error {
		return comm.Connect(o)
	})
	if err != nil {
		return nil, err
	}

	// Wait for the context to end and then disconnect
	go func() {
		<-ctx.Done()
		comm.Disconnect()
	}()

	return comm, err
}
