package shardmaster

import (
	"math"
	"sync"
	"time"

	"../labgob"
	"../labrpc"
	"../raft"
)

type ShardMaster struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	// Your data here.
	configs []Config // indexed by config num

	chMap   map[int]chan Op
	cid2Seq map[int64]int
	killCh  chan bool
}

type Op struct {
	// Your data here.
	OpType string
	Args   interface{} // could be JoinArgs, LeaveArgs, MoveArgs and QueryArgs, in reply it could be config
	Cid    int64
	SeqNum int
}

func (sm *ShardMaster) Join(args *JoinArgs, reply *JoinReply) {
	// Your code here.
	originOp := Op{"Join", *args, args.Cid, args.SeqNum}
	reply.WrongLeader = sm.templateHandler(originOp)
}

func (sm *ShardMaster) Leave(args *LeaveArgs, reply *LeaveReply) {
	// Your code here.
	originOp := Op{"Leave", *args, args.Cid, args.SeqNum}
	reply.WrongLeader = sm.templateHandler(originOp)
}

func (sm *ShardMaster) Move(args *MoveArgs, reply *MoveReply) {
	// Your code here.
	originOp := Op{"Move", *args, args.Cid, args.SeqNum}
	reply.WrongLeader = sm.templateHandler(originOp)
}

// different logic
func (sm *ShardMaster) Query(args *QueryArgs, reply *QueryReply) {
	// Your code here.
	reply.WrongLeader = true
	//
	// error: args.SeqNum ==-1 确保不会执行，如果写args.SeqNum,那么会放入对应的seqNum的index里面发生占用
	originOp := Op{"Query", *args, args.Cid, -1}
	reply.WrongLeader = sm.templateHandler(originOp)
	if !reply.WrongLeader {
		sm.mu.Lock()
		if args.Num >= 0 && args.Num < len(sm.configs) {
			reply.Config = sm.configs[args.Num]
		} else {
			reply.Config = sm.configs[len(sm.configs)-1]
		}
		sm.mu.Unlock()
	}
}

func (sm *ShardMaster) templateHandler(originOp Op) bool {
	wrongLeader := true
	index, _, isLeader := sm.rf.Start(originOp)
	if !isLeader {
		return wrongLeader
	}
	ch := sm.getCh(index, true)
	op := sm.beNotified(ch, index)
	if equalOp(op, originOp) {
		wrongLeader = false
	}
	return wrongLeader
}

func (sm *ShardMaster) beNotified(ch chan Op, index int) Op {
	select {
	case notifyArg := <-ch:
		close(ch)
		sm.mu.Lock()
		delete(sm.chMap, index)
		sm.mu.Unlock()
		return notifyArg
	case <-time.After(time.Duration(600) * time.Millisecond):
		return Op{}
	}
}

func equalOp(a Op, b Op) bool {
	return a.SeqNum == b.SeqNum && a.Cid == b.Cid && a.OpType == b.OpType
}

func (sm *ShardMaster) Kill() {
	sm.rf.Kill()
	sm.killCh <- true
}

// needed by shardkv tester
func (sm *ShardMaster) Raft() *raft.Raft {
	return sm.rf
}

func (sm *ShardMaster) getCh(idx int, createIfNotExists bool) chan Op {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if _, ok := sm.chMap[idx]; !ok {
		if !createIfNotExists {
			return nil
		}
		sm.chMap[idx] = make(chan Op, 1)
	}
	return sm.chMap[idx]
}

func (sm *ShardMaster) updateConfig(opType string, args interface{}) {
	config := sm.createNextConfig()
	switch opType {
	case "Join":
		joinArgs := args.(JoinArgs)
		for gid, servers := range joinArgs.Servers {
			newservers := make([]string, len(servers))
			copy(newservers, servers)
			config.Groups[gid] = newservers
			sm.rebalance(&config, opType, gid)
		}
	case "Leave":
		leaveArgs := args.(LeaveArgs)
		for _, gid := range leaveArgs.GIDs {
			delete(config.Groups, gid)
			sm.rebalance(&config, opType, gid)
		}
	case "Move":
		moveArgs := args.(MoveArgs)
		if _, exist := config.Groups[moveArgs.GID]; !exist {
			return
		} else {
			config.Shards[moveArgs.Shard] = moveArgs.GID
		}
	}
	sm.configs = append(sm.configs, config)
}

func (sm *ShardMaster) createNextConfig() Config {
	lastconfig := sm.configs[len(sm.configs)-1]
	config := Config{
		Num:    lastconfig.Num + 1,
		Shards: lastconfig.Shards,
		Groups: make(map[int][]string),
	}
	for gid, servers := range lastconfig.Groups {
		config.Groups[gid] = append([]string{}, servers...)
	}
	return config
}

func (sm *ShardMaster) rebalance(config *Config, opType string, gid int) {
	shardCount := sm.groupByGid(config)
	switch opType {
	case "Join":
		// 最小化的分配config
		// 移动，gid的大小和shared的大小
		// 没有进行分配的shared重新进行分配
		avg := NShards / len(config.Groups)
		for i := 0; i < avg; i++ {
			maxGid := sm.getMaxShardGid(shardCount)
			config.Shards[shardCount[maxGid][0]] = gid
			shardCount[maxGid] = shardCount[maxGid][1:]
		}
	case "Leave":
		shardsArray, exists := shardCount[gid]
		if !exists {
			return
		}
		delete(shardCount, gid)
		if len(config.Groups) == 0 { // remove all gid
			config.Shards = [NShards]int{}
			return
		}
		for _, v := range shardsArray {
			// 找到当前gid中最小的那个
			minGid := sm.getMinShardGid(shardCount)
			config.Shards[v] = minGid
			shardCount[minGid] = append(shardCount[minGid], v)
		}
	}
}

func (sm *ShardMaster) groupByGid(cfg *Config) map[int][]int {
	shardsCount := map[int][]int{}
	for k, _ := range cfg.Groups {
		shardsCount[k] = []int{}
	}
	for k, v := range cfg.Shards {
		shardsCount[v] = append(shardsCount[v], k)
	}
	return shardsCount
}
func (sm *ShardMaster) getMaxShardGid(shardsCount map[int][]int) int {
	max := -1
	var gid int
	for k, v := range shardsCount {
		if max < len(v) {
			max = len(v)
			gid = k
		}
	}
	return gid
}
func (sm *ShardMaster) getMinShardGid(shardsCount map[int][]int) int {
	min := math.MaxInt32
	var gid int
	for k, v := range shardsCount {
		if min > len(v) {
			min = len(v)
			gid = k
		}
	}
	return gid
}

func send(ch chan Op, op Op) {
	select {
	case <-ch:
	default:
	}
	ch <- op
}

func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister) *ShardMaster {
	sm := new(ShardMaster)
	sm.me = me
	sm.configs = make([]Config, 1)
	sm.configs[0].Groups = map[int][]string{}
	labgob.Register(Op{})
	labgob.Register(JoinArgs{})
	labgob.Register(LeaveArgs{})
	labgob.Register(MoveArgs{})
	labgob.Register(QueryArgs{})
	sm.applyCh = make(chan raft.ApplyMsg)
	sm.rf = raft.Make(servers, me, persister, sm.applyCh)
	// Your code here.
	sm.chMap = make(map[int]chan Op)
	sm.cid2Seq = make(map[int64]int)
	sm.killCh = make(chan bool, 1)
	go sm.applier()
	return sm
}

func (sm *ShardMaster) applier() {
	for {
		select {
		case <-sm.killCh:
			return
		case applyMsg := <-sm.applyCh:
			if !applyMsg.CommandValid {
				continue
			}
			op := applyMsg.Command.(Op)
			index := applyMsg.CommandIndex
			// 1. 检查幂等性并且不是snapshot
			sm.mu.Lock()
			seqNum, ok := sm.cid2Seq[op.Cid]
			//
			// error :query seqNumber==-1 so eliminate query
			//
			if op.SeqNum >= 0 && (!ok || seqNum < op.SeqNum) {
				sm.updateConfig(op.OpType, op.Args)
				sm.cid2Seq[op.Cid] = op.SeqNum
			}
			sm.mu.Unlock()
			// 2. 将op放入对应的chan中
			if ch := sm.getCh(index, false); ch != nil {
				send(ch, op)
			}
		}
	}
}
