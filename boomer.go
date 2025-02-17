package hrp

import (
	"time"

	"github.com/jinzhu/copier"
	"github.com/rs/zerolog/log"

	"github.com/httprunner/hrp/internal/boomer"
	"github.com/httprunner/hrp/internal/ga"
)

func NewBoomer(spawnCount int, spawnRate float64) *HRPBoomer {
	b := &HRPBoomer{
		Boomer: boomer.NewStandaloneBoomer(spawnCount, spawnRate),
		debug:  false,
	}
	return b
}

type HRPBoomer struct {
	*boomer.Boomer
	debug bool
}

// SetDebug configures whether to log HTTP request and response content.
func (b *HRPBoomer) SetDebug(debug bool) *HRPBoomer {
	b.debug = debug
	return b
}

// Run starts to run load test for one or multiple testcases.
func (b *HRPBoomer) Run(testcases ...ITestCase) {
	event := ga.EventTracking{
		Category: "RunLoadTests",
		Action:   "hrp boom",
	}
	// report start event
	go ga.SendEvent(event)
	// report execution timing event
	defer ga.SendEvent(event.StartTiming("execution"))

	var taskSlice []*boomer.Task
	for _, iTestCase := range testcases {
		testcase, err := iTestCase.ToTestCase()
		if err != nil {
			panic(err)
		}
		cfg := testcase.Config.ToStruct()
		err = initParameterIterator(cfg, "boomer")
		if err != nil {
			panic(err)
		}
		task := b.convertBoomerTask(testcase)
		taskSlice = append(taskSlice, task)
	}
	b.Boomer.Run(taskSlice...)
}

func (b *HRPBoomer) convertBoomerTask(testcase *TestCase) *boomer.Task {
	hrpRunner := NewRunner(nil).SetDebug(b.debug)
	config := testcase.Config.ToStruct()
	return &boomer.Task{
		Name:   config.Name,
		Weight: config.Weight,
		Fn: func() {
			runner := hrpRunner.newCaseRunner(testcase)

			testcaseSuccess := true       // flag whole testcase result
			var transactionSuccess = true // flag current transaction result

			cfg := testcase.Config.ToStruct()
			caseConfig := &TConfig{}
			// copy config to avoid data racing
			if err := copier.Copy(caseConfig, cfg); err != nil {
				log.Error().Err(err).Msg("copy config data failed")
			}
			// iterate through all parameter iterators and update case variables
			for _, it := range caseConfig.ParametersSetting.Iterators {
				if it.HasNext() {
					caseConfig.Variables = mergeVariables(it.Next(), caseConfig.Variables)
				}
			}
			startTime := time.Now()
			for index, step := range testcase.TestSteps {
				stepData, err := runner.runStep(index, caseConfig)
				if err != nil {
					// step failed
					var elapsed int64
					if stepData != nil {
						elapsed = stepData.elapsed
					}
					b.RecordFailure(step.Type(), step.Name(), elapsed, err.Error())

					// update flag
					testcaseSuccess = false
					transactionSuccess = false

					if runner.hrpRunner.failfast {
						log.Error().Msg("abort running due to failfast setting")
						break
					}
					log.Warn().Err(err).Msg("run step failed, continue next step")
					continue
				}

				// step success
				if stepData.stepType == stepTypeTransaction {
					// transaction
					// FIXME: support nested transactions
					if stepData.elapsed != 0 { // only record when transaction ends
						b.RecordTransaction(stepData.name, transactionSuccess, stepData.elapsed, 0)
						transactionSuccess = true // reset flag for next transaction
					}
				} else if stepData.stepType == stepTypeRendezvous {
					// rendezvous
					// TODO: implement rendezvous in boomer
				} else {
					// request or testcase step
					b.RecordSuccess(step.Type(), step.Name(), stepData.elapsed, stepData.contentSize)
				}
			}
			endTime := time.Now()

			// report duration for transaction without end
			for name, transaction := range runner.transactions {
				if len(transaction) == 1 {
					// if transaction end time not exists, use testcase end time instead
					duration := endTime.Sub(transaction[transactionStart])
					b.RecordTransaction(name, transactionSuccess, duration.Milliseconds(), 0)
				}
			}

			// report testcase as a whole Action transaction, inspired by LoadRunner
			b.RecordTransaction("Action", testcaseSuccess, endTime.Sub(startTime).Milliseconds(), 0)
		},
	}
}
