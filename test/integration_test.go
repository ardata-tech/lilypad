package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/lilypad-tech/lilypad/pkg/data"
	"github.com/lilypad-tech/lilypad/pkg/executor/noop"
	"github.com/lilypad-tech/lilypad/pkg/jobcreator"
	"github.com/lilypad-tech/lilypad/pkg/mediator"
	optionsfactory "github.com/lilypad-tech/lilypad/pkg/options"
	"github.com/lilypad-tech/lilypad/pkg/solver"
	"github.com/lilypad-tech/lilypad/pkg/system"
	"github.com/lilypad-tech/lilypad/pkg/web3"
	"github.com/stretchr/testify/assert"
	traceNoop "go.opentelemetry.io/otel/trace/noop"
)

type testOptions struct {
	moderationChance int
	executor         noop.NoopExecutorOptions
}

func getMediator(
	t *testing.T,
	systemContext *system.CommandContext,
	options testOptions,
) (*mediator.Mediator, error) {
	mediatorOptions := optionsfactory.NewMediatorOptions()
	mediatorOptions.Web3.PrivateKey = os.Getenv("MEDIATOR_PRIVATE_KEY")
	if mediatorOptions.Web3.PrivateKey == "" {
		return nil, fmt.Errorf("MEDIATOR_PRIVATE_KEY is not defined")
	}
	mediatorOptions, err := optionsfactory.ProcessMediatorOptions(mediatorOptions, "dev")
	if err != nil {
		return nil, err
	}

	noopTracer := traceNoop.NewTracerProvider().Tracer(system.GetOTelServiceName(system.MediatorService))
	web3SDK, err := web3.NewContractSDK(systemContext.Ctx, mediatorOptions.Web3, noopTracer)
	if err != nil {
		return nil, err
	}

	options.executor.BadActor = false

	executor, err := noop.NewNoopExecutor(options.executor)
	if err != nil {
		return nil, err
	}

	return mediator.NewMediator(mediatorOptions, web3SDK, executor)
}

func getJobCreatorOptions(options testOptions) (jobcreator.JobCreatorOptions, error) {
	jobCreatorOptions := optionsfactory.NewJobCreatorOptions()
	jobCreatorOptions.Web3.PrivateKey = os.Getenv("JOB_CREATOR_PRIVATE_KEY")
	if jobCreatorOptions.Web3.PrivateKey == "" {
		return jobCreatorOptions, fmt.Errorf("JOB_CREATOR_PRIVATE_KEY is not defined")
	}
	ret, err := optionsfactory.ProcessJobCreatorOptions(jobCreatorOptions, []string{
		// this should point to the shortcut
		"cowsay:v0.0.4",
	}, "dev")

	if err != nil {
		return ret, err
	}

	jobCreatorOptions.Mediation.CheckResultsPercentage = options.moderationChance
	return ret, nil
}

func testStackWithOptions(
	t *testing.T,
	commandCtx *system.CommandContext,
	options testOptions,
) (*jobcreator.RunJobResults, error) {
	// give the solver server a chance to boot before we get all the websockets
	// up and trying to connect to it
	time.Sleep(20000 * time.Millisecond)

	mediator, err := getMediator(t, commandCtx, options)
	if err != nil {
		return nil, err
	}

	mediator.Start(commandCtx.Ctx, commandCtx.Cm)

	jobCreatorOptions, err := getJobCreatorOptions(options)
	if err != nil {
		return nil, err
	}

	result, err := jobcreator.RunJob(commandCtx, jobCreatorOptions, func(evOffer data.JobOfferContainer) {

	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func TestNoModeration(t *testing.T) {
	commandCtx := system.NewTestingContext()
	defer commandCtx.Cleanup()

	message := "hello apples this is a message"

	executorOptions := noop.NewNoopExecutorOptions()
	executorOptions.Stdout = message

	result, err := testStackWithOptions(t, commandCtx, testOptions{
		moderationChance: 0,
		executor:         executorOptions,
	})

	assert.NoError(t, err, "there was an error running the job")
	assert.Equal(t, "QmbCi3yoKzckff24rUJML1ZesVb35cd2LUNMiMYksEGkWv", result.Result.DataID, "the data ID was correct")

	localPath := solver.GetDownloadsFilePath(result.Result.DealID)

	fmt.Printf("result --------------------------------------\n")
	spew.Dump(localPath)
}
