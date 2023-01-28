package shardmaster

//
// Shardmaster clerk.
//

import (
	"crypto/rand"
	"math/big"
	"time"

	"../labrpc"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// Your data here.
	id         int64
	seqNum     int
	lastLeader int
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers

	// Your code here.
	ck.id = nrand()
	ck.lastLeader = 0
	ck.seqNum = 0

	return ck
}

// Query(num) -> fetch Config # num, or latest config if num==-1.
func (ck *Clerk) Query(num int) Config {
	args := &QueryArgs{}
	// Your code here.
	args.Num = num
	for {
		// try each known server.
		var reply QueryReply
		ok := ck.servers[ck.lastLeader].Call("ShardMaster.Query", args, &reply)
		if ok && reply.WrongLeader == false {
			return reply.Config
		}

		ck.lastLeader = (ck.lastLeader + 1) % len(ck.servers)
		time.Sleep(100 * time.Millisecond)
	}
}

// Join(servers) -- add a set of groups (gid -> server-list mapping).
func (ck *Clerk) Join(servers map[int][]string) {
	args := &JoinArgs{}
	// Your code here.
	args.Servers = servers
	args.Cid = ck.id
	args.SeqNum = ck.seqNum
	ck.seqNum++

	for {
		// try each known server.
		var reply JoinReply
		ok := ck.servers[ck.lastLeader].Call("ShardMaster.Join", args, &reply)
		if ok && reply.WrongLeader == false {
			return
		}

		ck.lastLeader = (ck.lastLeader + 1) % len(ck.servers)
		time.Sleep(100 * time.Millisecond)
	}
}

//Leave(gids) -- delete a set of groups.
func (ck *Clerk) Leave(gids []int) {
	args := &LeaveArgs{}
	// Your code here.
	args.GIDs = gids
	args.Cid = ck.id
	args.SeqNum = ck.seqNum
	ck.seqNum++

	for {
		// try each known server.
		var reply LeaveReply
		ok := ck.servers[ck.lastLeader].Call("ShardMaster.Leave", args, &reply)
		if ok && reply.WrongLeader == false {
			return
		}

		ck.lastLeader = (ck.lastLeader + 1) % len(ck.servers)
		time.Sleep(100 * time.Millisecond)
	}
}

// Move(shard, gid) -- hand off one shard from current owner to gid.
func (ck *Clerk) Move(shard int, gid int) {
	args := &MoveArgs{}
	// Your code here.
	args.Shard = shard
	args.GID = gid
	args.Cid = ck.id
	args.SeqNum = ck.seqNum
	ck.seqNum++

	for {
		// try each known server.
		var reply MoveReply
		ok := ck.servers[ck.lastLeader].Call("ShardMaster.Move", args, &reply)
		if ok && reply.WrongLeader == false {
			return
		}

		ck.lastLeader = (ck.lastLeader + 1) % len(ck.servers)
		time.Sleep(100 * time.Millisecond)
	}
}
