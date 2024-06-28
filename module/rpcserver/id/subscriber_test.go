/*
Created by guoxin in 2022/11/17 2:48 PM
*/
package id

import (
	"fmt"
	"strings"
	"testing"
)

func TestIdentity(t *testing.T) {
	id := "143010000000001"
	index := strings.Index(id, "00")
	fmt.Println(id[0:index])

	i := strings.Index(id, id[1:index])
	fmt.Println(i)

	i = strings.Index("321312312312", id[0:index])
	fmt.Println(i)

	id1 := "1XX43010000000001"
	index1 := strings.Index(id1, "00")
	fmt.Println(id1[3:index1])
}

func TestSubscriberId_GetType(t *testing.T) {
	tests := []struct {
		name string
		s    SubscriberId
		want string
	}{
		{
			name: "正常",
			s: func() SubscriberId {
				subscriberId, err := NewSubscriberId("150000")
				if err != nil {
					panic(err)
				}
				return subscriberId
			}(),
			want: "1",
		},
		{
			name: "正常",
			s: func() SubscriberId {
				subscriberId, err := NewSubscriberId("143000")
				if err != nil {
					panic(err)
				}
				return subscriberId
			}(),
			want: "1",
		},
		{
			name: "正常",
			s: func() SubscriberId {
				subscriberId, err := NewSubscriberId("243000")
				if err != nil {
					panic(err)
				}
				return subscriberId
			}(),
			want: "2",
		},
		{
			name: "正常",
			s: func() SubscriberId {
				subscriberId, err := NewSubscriberId("343000")
				if err != nil {
					panic(err)
				}
				return subscriberId
			}(),
			want: "3",
		},
		{
			name: "正常",
			s: func() SubscriberId {
				subscriberId, err := NewSubscriberId("443000")
				if err != nil {
					panic(err)
				}
				return subscriberId
			}(),
			want: "4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.GetType(); got != tt.want {
				t.Errorf("GetType() = %v, want %v", got, tt.want)
			}
		})
	}
}
