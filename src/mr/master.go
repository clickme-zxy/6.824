/*
	the master sever:
	1. given the basic information and the master will work
		(1) nreduce
		(2) nmap
		(3) maptask_chan、maptask_records(map structure)
		(4) reducetask_chan、reducetask_records(map structure)
		(5) state
		result
		(6) map_result_path \reduce_result_path
*/
package mr

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

type State int

const TimeLimit = 10
const (
	// shuttle
	TaskWait State = iota
	// the job over
	JobOver
	// for the map task
	MapPhase
	// for the reduce task
	ReducePhase
)

type MapTask struct {
	Id       int    // the id for the map task
	FilePath string // the filepath for the map task
}

type ReduceTask struct {
	Id        int      // the id for the reduce task
	FilePaths []string // the filepaths for the reduce task
}

type Master struct {
	// the state
	Phase State
	// the reduce task and the map task
	NReduce int
	NMap    int

	// this is for the listen of the task state
	MapTaskState    map[int]bool
	ReduceTaskState map[int]bool
	// this is for the task to give the worker
	MapTaskQueue    chan MapTask
	ReduceTaskQueue chan ReduceTask

	// map response with the filepath
	MapOutPaths [][]Response
	// after shuttle it will become the reduceoutpath
	ReduceOutPaths []string

	Mut sync.Mutex
}

/*
	the worker ask the master for get task
	if the master is in the mapstate:
		it will give the worker a map task
	if the master is in the reducestate:
		it will give the worker a reduce task
	if the master is in the shuttle:
		the worker must wait until the over

	-----------------------------------------
	info:
		the RPCARGS ans given the RPCREPLY
	we use the select to 无阻塞的读取信息
*/
func (m *Master) GetTask(args *RPCArgs, reply *RPCReply) error {
	m.Mut.Lock()
	reply.Phase = m.Phase
	reply.Nreduce = m.NReduce
	m.Mut.Unlock()

	reply.HoldTask = true
	switch reply.Phase {
	case MapPhase:
		// 使用select 可以无堵塞读取信道
		select {
		case reply.MapTask = <-m.MapTaskQueue:
			go func(reply RPCReply) {
				time.Sleep(TimeLimit * time.Second)
				m.Mut.Lock()
				if !m.MapTaskState[reply.MapTask.Id] {
					m.MapTaskQueue <- reply.MapTask
				}
				fmt.Printf("Map failed with time over: %v\n", reply.MapTask.Id)
				m.Mut.Unlock()
			}(*reply)
		default:
			reply.HoldTask = false
		}
	case ReducePhase:
		select {
		case reply.ReduceTask = <-m.ReduceTaskQueue:
			go func(reply RPCReply) {
				time.Sleep(TimeLimit * time.Second)
				m.Mut.Lock()
				if !m.ReduceTaskState[reply.ReduceTask.Id] {
					m.ReduceTaskQueue <- reply.ReduceTask
					fmt.Printf("Reduce failed with time over: %v\n", reply.ReduceTask.Id)
				}
				m.Mut.Unlock()
			}(*reply)
		default:
			reply.HoldTask = false
		}
	case TaskWait:
		reply.HoldTask = false
	case JobOver:
		reply.HoldTask = false
	}

	return nil
}

/*
1. map 或者 reduce任务完成，则记录中间文件路径，并更新任务状态
2. 当所有map完成之后，调用shuffle()函数,根据中间文件路径数据生成reduce任务，进入reducePhase阶段
3. reduce任务是否全部完成，只需判断最终文件路径的个数是否和任务状态个数相等，不等使用nreduce,因为不一定会生成
	nreduce个reduce任务
*/
func (m *Master) Update(args *RPCArgs, reply *RPCReply) error {
	m.Mut.Lock()
	switch m.Phase {
	case MapPhase:
		if !m.MapTaskState[args.Id] {
			m.MapTaskState[args.Id] = true
			m.MapOutPaths = append(m.MapOutPaths, args.OutPaths)
		}
		if len(m.MapOutPaths) == m.NMap {
			m.Phase = TaskWait
			m.shuffle()
			m.Phase = ReducePhase
		}
	case ReducePhase:
		if !m.ReduceTaskState[args.Id] {
			m.ReduceTaskState[args.Id] = true
			m.ReduceOutPaths = append(m.ReduceOutPaths, args.OutPaths[0].OutPath)
		}
		if len(m.ReduceOutPaths) == len(m.ReduceTaskState) {
			m.Phase = JobOver
		}
	}
	m.Mut.Unlock()
	return nil
}

func (m *Master) shuffle() error {
	for i := 0; i < m.NReduce; i++ {
		reduceTask := ReduceTask{Id: i}
		for _, mapoutpath := range m.MapOutPaths {
			for _, path := range mapoutpath {
				if path.Id == i {
					reduceTask.FilePaths = append(reduceTask.FilePaths, path.OutPath)
				}
			}
		}
		if len(reduceTask.FilePaths) != 0 {
			m.ReduceTaskQueue <- reduceTask
			m.ReduceTaskState[i] = false
		}
	}
	return nil
}

func (m *Master) Alive(args *RPCArgs, reply *RPCReply) error {
	// if err without problem
	reply.Phase = m.Phase
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (m *Master) server() {
	rpc.Register(m)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := masterSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
//
func (m *Master) Done() bool {
	defer m.Mut.Unlock()
	m.Mut.Lock()
	return m.Phase == JobOver
}

//
// create a Master.
// main/mrmaster.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeMaster(files []string, nReduce int) *Master {
	// initialize master variables
	m := Master{}
	m.NReduce = nReduce
	m.NMap = len(files)
	m.MapTaskState = make(map[int]bool)
	m.ReduceTaskState = make(map[int]bool)
	m.MapTaskQueue = make(chan MapTask, len(files))
	m.ReduceTaskQueue = make(chan ReduceTask, nReduce)
	m.Phase = MapPhase
	// create the map task
	for i, file := range files {
		mapTask := MapTask{i, file}
		m.MapTaskState[i] = false
		m.MapTaskQueue <- mapTask
	}
	m.server()
	return &m
}
