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
		p            string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "省局订阅，省局交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "14000000000",
			},
			want: true,
		},
		{
			name: "省局订阅，外省局交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "15000000000",
			},
			want: false,
		},
		{
			name: "省局订阅，市局交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "14010000000",
			},
			want: true,
		},
		{
			name: "省局订阅，外省市局交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "15010000000",
			},
			want: false,
		},
		{
			name: "省局订阅，本省市区交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "14010100000",
			},
			want: true,
		},
		{
			name: "省局订阅，外省市区交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "15010100000",
			},
			want: false,
		},
		// ==============================外部政府部门==============================
		{
			name: "省局订阅，外部政府部门省交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "2xx4000000000000000",
			},
			want: true,
		},
		{
			name: "省局订阅，外部政府部门市交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "2xx4010000000000000",
			},
			want: true,
		},
		{
			name: "省局订阅，外部政府部门区交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "2xx4010010000000000",
			},
			want: true,
		},
		{
			name: "省局订阅，外部政府部门外省交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "2xx5010010000000000",
			},
			want: false,
		},
		// ==============================企业==============================
		{
			name: "省局订阅，企业省交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "3xx4000000000000000",
			},
			want: true,
		},
		{
			name: "省局订阅，企业市交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "3xx4010000000000000",
			},
			want: true,
		},
		{
			name: "省局订阅，企业区交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "3xx4010010000000000",
			},
			want: true,
		},
		{
			name: "省局订阅，企业外省交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "3xx5010010000000000",
			},
			want: false,
		},
		// ==============================自然人==============================
		{
			name: "省局订阅，自然人区交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "4400623199206036666",
			},
			want: true,
		},
		{
			name: "省局订阅，自然人区交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14000000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "4510623199206036666",
			},
			want: false,
		},
		// ==============================企业订阅其他==============================
		{
			name: "企业订阅，省局交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3xx401001MA01R2FT36")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "14000000000",
			},
			want: false,
		},
		{
			name: "企业订阅，政府交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3xx401001MA01R2FT36")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "2xx401001MA01R2FT36",
			},
			want: false,
		},
		{
			name: "企业订阅，企业自己交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3xx401001MA01R2FT36")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "3xx401001MA01R2FT36",
			},
			want: true,
		},
		{
			name: "企业订阅，其他企业交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3xx401001MA01R2FT36")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "3xx401001MA01R2FA36",
			},
			want: false,
		},

		// ==============================other==============================
		{
			name: "重庆税务省局订阅，重庆省局下辖区县交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("15002000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "15001111111",
			},
			want: false,
		},
		{
			name: "税务省局订阅，企业交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "2XX4301111111123456",
			},
			want: true,
		},
		{
			name: "税务省局订阅，企业交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "3XX4301111111123456",
			},
			want: true,
		},
		{
			name: "税务省局订阅，自然人交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("14301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "4430111111112345600",
			},
			want: true,
		},
		{
			name: "外部政府部门订阅，省局交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "14301111111",
			},
			want: false,
		},
		{
			name: "外部政府部门订阅，外部政府部门交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "2XX4301111111123456",
			},
			want: true,
		},
		{
			name: "外部政府部门订阅，企业交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "3XX4301111111123456",
			},
			want: false,
		},
		{
			name: "外部政府部门订阅，自然人交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "4430111111112345600",
			},
			want: false,
		},

		{
			name: "企业订阅，省局交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "14301111111",
			},
			want: false,
		},
		{
			name: "企业订阅，外部政府部门交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "2XX4301111111123456",
			},
			want: false,
		},
		{
			name: "企业订阅，企业交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("3XX4301111111123456")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "3XX4301111111123456",
			},
			want: true,
		},
		{
			name: "企业订阅，自然人交易",
			args: args{
				subscriberId: func() SubscriberId {
					subscriberId, err := NewSubscriberId("2XX4301000000")
					if err != nil {
						panic(err)
					}
					return subscriberId
				}(),
				p: "4430111111112345600",
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
