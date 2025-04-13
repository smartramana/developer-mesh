package internal

// This file contains adapter method stubs that need to be implemented
// for the packages that were missing the ExecuteAction and IsSafeOperation methods

/*
Add this to harness.Adapter:

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// IsSafeOperation determines if an operation is safe to perform
func (a *Adapter) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	return true, nil
}

Add this to sonarqube.Adapter:

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// IsSafeOperation determines if an operation is safe to perform
func (a *Adapter) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	return true, nil
}

Add this to xray.Adapter:

// ExecuteAction executes an action with context awareness
func (a *Adapter) ExecuteAction(ctx context.Context, contextID string, action string, params map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

// IsSafeOperation determines if an operation is safe to perform
func (a *Adapter) IsSafeOperation(operation string, params map[string]interface{}) (bool, error) {
	return true, nil
}
*/
