// Template for service tests
// Usage: Copy this template when writing new service tests

package services

import (
	"context"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock{Service} is a mock implementation of {Service}Interface
type Mock{Service} struct {
	mock.Mock
}

func (m *Mock{Service}) {Method}(ctx context.Context, params ...interface{}) (interface{}, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0), args.Error(1)
}

func Test{Service}_{Method}(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Mock{Dependency})
		input   {InputType}
		want    {OutputType}
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful {operation}",
			setup: func(mockDep *Mock{Dependency}) {
				mockDep.On("{Method}", mock.Anything, mock.Anything).
					Return(&{ReturnType}{}, nil)
			},
			input: {InputType}{
				// TODO: Fill test input
			},
			want: {OutputType}{
				// TODO: Fill expected output
			},
			wantErr: false,
		},
		{
			name: "handles error from dependency",
			setup: func(mockDep *Mock{Dependency}) {
				mockDep.On("{Method}", mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("dependency error"))
			},
			input: {InputType}{
				// TODO: Fill test input
			},
			want:    nil,
			wantErr: true,
			errMsg:  "dependency error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockDep := new(Mock{Dependency})
			if tt.setup != nil {
				tt.setup(mockDep)
			}

			// Create service
			service := &{Service}{
				dependency: mockDep,
				logger:     observability.NewNoOpLogger(),
			}

			// Execute
			ctx := context.Background()
			got, err := service.{Method}(ctx, tt.input)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			// Verify all expectations were met
			mockDep.AssertExpectations(t)
		})
	}
}