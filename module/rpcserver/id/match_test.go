/*
Created by guoxin in 2022/11/17 5:10 PM
*/
package id

import (
	"chainmaker.org/chainmaker/protocol/v2/test"
	"testing"
)

func TestIdentityMatchImpl_Match(t *testing.T) {
	type args struct {
		subscriberId SubscriberId
		p            ParticipantId
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "税务省局订阅，省局交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("14301111111")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: true,
		},
		{
			name: "重庆税务省局订阅，重庆省局下辖区县交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("15000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("15001111111")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: true,
		},
		{
			name: "重庆税务省局订阅，重庆省局下辖区县交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("15001000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("15001111111")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: true,
		},
		{
			name: "重庆税务省局订阅，重庆省局下辖区县交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("15002000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("15001111111")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: false,
		},
		{
			name: "税务省局订阅，外部政府部门交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("2XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: true,
		},
		{
			name: "税务省局订阅，企业交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("3XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: true,
		},
		{
			name: "税务省局订阅，自然人交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("4430111111112345600")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: true,
		},
		{
			name: "外部政府部门订阅，省局交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("14301111111")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: false,
		},
		{
			name: "外部政府部门订阅，外部政府部门交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("2XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: true,
		},
		{
			name: "外部政府部门订阅，企业交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("3XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: false,
		},
		{
			name: "外部政府部门订阅，自然人交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("4430111111112345600")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: false,
		},

		{
			name: "企业订阅，省局交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("14301111111")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: false,
		},
		{
			name: "企业订阅，外部政府部门交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("2XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: false,
		},
		{
			name: "企业订阅，企业交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("3XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: true,
		},
		{
			name: "企业订阅，自然人交易清分",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: func() ParticipantId {
					participant, err := NewParticipantId("4430111111112345600")
					if err != nil {
						panic(err)
					}
					return participant
				}(),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := IdentityMatchImpl{log: test.NewTestLogger(t)}
			if got := i.Match(tt.args.subscriberId, tt.args.p); got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}
