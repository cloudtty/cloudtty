package main

import "testing"

// ssh root@cluster1.node1@10.6.111.111
// ssh root@10.5.2.1@10.6.111.111
// ssh cluster1.ns1.app1@10.6.111.111

func Test_checkUser(t *testing.T) {
	type args struct {
		user string
	}
	type want struct {
		username string
		usertype UserLoginType
		target   string
		err      error
	}

	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "access by name",
			args: args{user: "root@cluster1.node1"},
			want: want{
				username: "root",
				usertype: AccessName,
				target:   "cluster1.node1",
				err:      nil,
			},
		},
		{
			name: "access by IP",
			args: args{user: "root@10.2.2.2"},
			want: want{
				username: "root",
				usertype: IPAddress,
				target:   "10.2.2.2",
				err:      nil,
			},
		},
	}
	t.Run("normal case", func(t *testing.T) {
		for _, test := range tests {
			usertype, user, target, err := checkUserForNode(test.args.user)
			if err != nil {
				t.Errorf("want err nil")
			}
			if usertype != test.want.usertype {
				t.Errorf("want user type is %v, but actual is %v", test.want.usertype, usertype)
			}
			if user != test.want.username {
				t.Errorf("want user is %s, but actual is %s", test.want.username, user)
			}

			if target != test.want.target {
				t.Errorf("want target is %s, but actual is %s", test.want.target, target)
			}
		}

	})
}
