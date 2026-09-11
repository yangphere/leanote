package db

import (
	"context"
	"fmt"
)

// ExecuteActionTokenMutation runs a sensitive user mutation and one-time token
// consumption inside a supplied transaction. Without a transaction boundary
// it does not attempt either operation: there is no durable compensation
// receipt for these security-sensitive pairs.
func ExecuteActionTokenMutation(ctx context.Context, transaction TransactionRunner, update func(context.Context) error, consume func(context.Context) error) error {
	if transaction == nil {
		return fmt.Errorf("action token mutation: %w", ErrPartialWrite)
	}
	if update == nil || consume == nil {
		return fmt.Errorf("action token mutation: invalid operation")
	}
	return transaction(ctx, func(txCtx context.Context) error {
		if err := update(txCtx); err != nil {
			return fmt.Errorf("apply action token mutation: %w", err)
		}
		if err := consume(txCtx); err != nil {
			return fmt.Errorf("consume action token: %w", err)
		}
		return nil
	})
}

// RunActionTokenMutation supplies Mongo's transaction boundary to the generic
// mutation. A missing/unavailable transaction capability fails closed rather
// than replaying the user update outside a durable receipt.
func RunActionTokenMutation(ctx context.Context, update func(context.Context) error, consume func(context.Context) error) error {
	if client == nil {
		return ExecuteActionTokenMutation(ctx, nil, update, consume)
	}
	session, err := client.StartSession()
	if err != nil {
		return fmt.Errorf("start action token transaction: %w", err)
	}
	defer session.EndSession(context.Background())
	runner := func(parent context.Context, apply func(context.Context) error) error {
		_, err := session.WithTransaction(parent, func(txCtx context.Context) (any, error) {
			return nil, apply(txCtx)
		})
		return err
	}
	return ExecuteActionTokenMutation(ctx, runner, update, consume)
}

// ExecutePasswordTokenMutation keeps the previous password-specific API as a
// compatibility wrapper for focused tests and callers.
func ExecutePasswordTokenMutation(ctx context.Context, transaction TransactionRunner, update func(context.Context) error, consume func(context.Context) error) error {
	return ExecuteActionTokenMutation(ctx, transaction, update, consume)
}

// RunPasswordTokenMutation keeps the previous password-specific API as a
// compatibility wrapper for focused tests and callers.
func RunPasswordTokenMutation(ctx context.Context, update func(context.Context) error, consume func(context.Context) error) error {
	return RunActionTokenMutation(ctx, update, consume)
}
