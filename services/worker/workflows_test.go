package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/mini4141/pay-by-bank-checkout/internal/flow"
)

func newEnv(t *testing.T) *testsuite.TestWorkflowEnvironment {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(PaymentWorkflow)
	env.RegisterActivity(&Activities{})
	return env
}

func TestPaymentWorkflowHappyPath(t *testing.T) {
	env := newEnv(t)
	var a *Activities
	env.OnActivity(a.ReserveFunds, mock.Anything, "pay_1").Return(nil)
	env.OnActivity(a.RiskCheck, mock.Anything, "pay_1").Return(true, nil)
	env.OnActivity(a.Debit, mock.Anything, "pay_1").Return(nil)
	env.OnActivity(a.SettleToMerchant, mock.Anything, "pay_1").Return(nil)
	env.OnActivity(a.DeliverWebhook, mock.Anything, "pay_1", eventSucceeded).Return(nil)

	env.ExecuteWorkflow(PaymentWorkflow, flow.PaymentInput{PaymentID: "pay_1"})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	v, err := env.QueryWorkflow(flow.QueryStatus)
	require.NoError(t, err)
	var status string
	require.NoError(t, v.Get(&status))
	require.Equal(t, flow.StatusSettled, status)
	env.AssertExpectations(t)
}

func TestDebitIsRetriedAfterOneError(t *testing.T) {
	env := newEnv(t)
	var a *Activities
	env.OnActivity(a.ReserveFunds, mock.Anything, "pay_2").Return(nil)
	env.OnActivity(a.RiskCheck, mock.Anything, "pay_2").Return(true, nil)
	env.OnActivity(a.Debit, mock.Anything, "pay_2").Return(errors.New("bank unavailable")).Once()
	env.OnActivity(a.Debit, mock.Anything, "pay_2").Return(nil).Once()
	env.OnActivity(a.SettleToMerchant, mock.Anything, "pay_2").Return(nil)
	env.OnActivity(a.DeliverWebhook, mock.Anything, "pay_2", eventSucceeded).Return(nil)

	env.ExecuteWorkflow(PaymentWorkflow, flow.PaymentInput{PaymentID: "pay_2"})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertNumberOfCalls(t, "Debit", 2)
}
