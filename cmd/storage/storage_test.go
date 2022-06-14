package storage

import (
	"context"
	"testing"
)

func TestNewStorage(t *testing.T) {
	tests := []struct {
		name              string
		uri               string
		s                 *Storage
		wantNilConnection bool
		wantErr           bool
	}{
		{
			name:              "Normal conditions",
			uri:               "postgres://postgres:example@localhost:5432",
			wantNilConnection: false,
			wantErr:           false,
		},
		{
			name:              "Empty uri",
			uri:               "",
			wantNilConnection: true,
			wantErr:           true,
		},
		{
			name:              "Parse error",
			uri:               "1234567890",
			wantNilConnection: true,
			wantErr:           true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewStorageConnection(context.Background(), tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewStorage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if s.Connection == nil && !tt.wantNilConnection {
				t.Errorf("Connection should not be equal nil")
				return
			}
			if s.Connection != nil && tt.wantNilConnection {
				t.Errorf("Connection should be equal nil")
				return
			}
		})
	}
}

func TestStorage_FindUserExists(t *testing.T) {
	uri := "postgres://postgres:example@localhost:5432"
	ts, err := NewStorageConnection(context.Background(), uri)
	if err != nil {
		t.Errorf("Failed to create new test storage connection, error: %v", err)
	}
	err = ts.CreateUser(&RegisterForm{
		Login:       "testExists",
		Password:    "1",
		TimeCreated: "Right now",
	})
	if err != nil {
		t.Errorf("Failed to create test case %v", err)
	}

	tests := []struct {
		name    string
		login   string
		want    bool
		wantErr bool
	}{
		{
			name:    "User not exists",
			login:   "testNotExists",
			want:    false,
			wantErr: false,
		},
		{
			name:    "User exists",
			login:   "testExists",
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ts.FindUserExists(tt.login)
			if (err != nil) != tt.wantErr {
				t.Errorf("Storage.FindUserExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Storage.FindUserExists() = %v, want %v", got, tt.want)
			}
		})
	}
	err = ts.DeleteUser("testExists")
	if err != nil {
		t.Errorf("Failed to clean test case %v", err)
	}
}

func TestStorage_FindLoginPass(t *testing.T) {
	uri := "postgres://postgres:example@localhost:5432"
	ts, err := NewStorageConnection(context.Background(), uri)
	if err != nil {
		t.Errorf("Failed to create new test storage connection, error: %v", err)
	}
	err = ts.CreateUser(&RegisterForm{
		Login:       "testExists",
		Password:    "1",
		TimeCreated: "Right now",
	})

	tests := []struct {
		name     string
		login    string
		password string
		want     bool
		wantErr  bool
	}{
		{
			name:     "User, password hash exists in base",
			login:    "testExists",
			password: "1",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "User, password hash not exists in base",
			login:    "testNotExists",
			password: "2",
			want:     false,
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ts.FindLoginPass(tt.login, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("Storage.FindLoginPass() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Storage.FindLoginPass() = %v, want %v", got, tt.want)
			}
		})
	}
	err = ts.DeleteUser("testExists")
	if err != nil {
		t.Errorf("Failed to clean test case %v", err)
	}
}

func TestStorage_CreateUser(t *testing.T) {
	uri := "postgres://postgres:example@localhost:5432"
	ts, err := NewStorageConnection(context.Background(), uri)
	if err != nil {
		t.Errorf("Failed to create new test storage connection, error: %v", err)
	}

	type args struct {
		rf *RegisterForm
	}
	tests := []struct {
		name    string
		s       *Storage
		args    args
		wantErr bool
	}{
		{
			name: "Normal",
			s:    ts,
			args: args{&RegisterForm{
				Login:       "test",
				Password:    "1",
				TimeCreated: "Right now",
			},
			},
			wantErr: false,
		},
		{
			name: "Normal again (same login should call err)",
			s:    ts,
			args: args{&RegisterForm{
				Login:       "test",
				Password:    "1",
				TimeCreated: "Right now",
			},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.s.CreateUser(tt.args.rf); (err != nil) != tt.wantErr {
				t.Errorf("Storage.CreateUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	err = ts.DeleteUser("test")
	if err != nil {
		t.Errorf("Failed to clean test case %v", err)
	}
}

func TestStorage_DeleteUser(t *testing.T) {
	uri := "postgres://postgres:example@localhost:5432"
	ts, err := NewStorageConnection(context.Background(), uri)
	if err != nil {
		t.Errorf("Failed to create new test storage connection, error: %v", err)
	}
	err = ts.CreateUser(&RegisterForm{
		Login:       "testExists",
		Password:    "1",
		TimeCreated: "Right now",
	})

	type args struct {
		login string
	}
	tests := []struct {
		name    string
		s       *Storage
		args    args
		wantErr bool
	}{
		{
			name: "User exists",
			s: ts,
			args: args{login: "testExists"},
			wantErr: false,
		},
		{
			name: "User not exists (should return no errors)",
			s: ts,
			args: args{login: "testExists"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.s.DeleteUser(tt.args.login); (err != nil) != tt.wantErr {
				t.Errorf("Storage.DeleteUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
