package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestModelUpdate(t *testing.T) {
	tests := []struct {
		name              string
		initialModel      model
		msg               tea.Msg
		expectQuit        bool
		expectedQuitState bool
	}{
		{
			name:              "quit on 'q'",
			initialModel:      initialModel(),
			msg:               tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
			expectQuit:        true,
			expectedQuitState: true,
		},
		{
			name:              "quit on ctrl+c",
			initialModel:      initialModel(),
			msg:               tea.KeyMsg{Type: tea.KeyCtrlC},
			expectQuit:        true,
			expectedQuitState: true,
		},
		{
			name:              "ignore other key",
			initialModel:      initialModel(),
			msg:               tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")},
			expectQuit:        false,
			expectedQuitState: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newModel, cmd := tt.initialModel.Update(tt.msg)
			m, ok := newModel.(model)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedQuitState, m.quitting)

			if tt.expectQuit {
				assert.NotNil(t, cmd)
				res := cmd()
				_, isQuit := res.(tea.QuitMsg)
				assert.True(t, isQuit)
			} else {
				assert.Nil(t, cmd)
			}
		})
	}
}

func TestModelView(t *testing.T) {
	tests := []struct {
		name     string
		model    model
		expected string
	}{
		{
			name:     "initial view",
			model:    initialModel(),
			expected: "lazytask (scaffold) - press 'q' or 'ctrl+c' to quit\n",
		},
		{
			name:     "quitting view",
			model:    model{quitting: true},
			expected: "Exiting lazytask...\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.model.View())
		})
	}
}
