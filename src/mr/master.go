package mr

import "log"
import "net"
import "os"
import "net/rpc"
import "net/http"

// master need to know the number of the worker tast and the reduce task
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
	Id int	// the id for the map task
	FilePath string  // the filepath for the map task
}

type ReduceTask struct {
	Id int	// the id for the reduce task
	FilePaths []string // the filepaths for the reduce task
}

type Master struct {
	// the state
	Phase State
	// the reduce task and the map task
	NReduce int
	NMap int

	// this is for the listen of the task state
	MapTaskState map[int]bool
	ReduceTaskState map[int]bool
	// this is for the task to give the worker
	MapTaskQueue chan MapTask
	ReduceTaskQueue chan ReduceTask

	// map response with the filepath
	MapOutPaths [][]Response
	// after shuttle it will become the reduceoutpath
	ReduceOutPaths []string

	Mut sync.Mutex
}

// Your code here -- RPC handlers for the worker to call.

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (m *Master) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

func (m *Master) GetTask(args *RPCArgs, reply *RPCReply) error {
	m.Mut.Lock()
	reply.Phase = m.Phase
	reply.Nreduce = m.Nreduce
	m.Mut.Unlock()
	
	// work向master申请任务
	reply.HoldTask = true
	switch reply.Phase {
		case MapPhase:
			// 使用select 可以无堵塞读取信道
			select {
				case reply.MapTask:=<-m.MapTaskQueue:
					// 开启携程判断是否超时
					go func(reply RPCReply){
						time.Sleep(TimeLimit*time.Second)
						m.Mut.Lock()
						if !m.MapTaskState[reply.MapTask.Id]{
							c.MapTaskQueue<-reply.MapTask
						}
						m.Mut.Unlock()
					}(*reply)
				default:
					reply.HoldTask = false
			}
		case ReducePhase:
			select{
				case reply.ReduceTask:=<-m.ReduceTaskQueue:
					go func(reply RPCReply){
						time.Sleep(TimeLimit*time.Second)
						m.Mut.Lock()
						if !m.ReduceTaskState[reply.ReduceTask.Id]{
							m.ReduceTaskQueue<-reply.ReduceTask
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
func (m *Master) UpdateState(args *RPCArgs, reply *RPCReply) error {
	m.Mut.Lock()
	switch m.Phase{
		case MapPhase:
			if !m.MapTaskState[args.Id]{
				m.MapTaskState[args.Id] = true
				m.MapOutPaths.append(m.MapOutPaths,args.OutPaths)
				// 有毛用，我已经锁起来了啦

			}
			if(len(m.MapOutPaths)==m.NMap){
				m.Phase = TaskWait
				// 排序操作开始
				m.shuffle()
				m.Phase = ReducePhrase
			}
		case ReducePhase:
			if !m.ReduceTaskState[args.Id] {
				m.ReduceOutPaths = append(m.ReduceOutPaths,args.OutPaths[0].OutPath)
				m.ReduceTaskState[args.Id] = true
			}
			if len(m.ReduceOutPaths)== len(m.ReduceTaskState) {
				m.Phase = JobOver
			}	
	}
	c.Mut.Unlock()
	return nil
}
/*
不一定所有的NReduce文件都存在，也就是不一定有Nreduce个任务
*/
func (m *Master) shuffle() error{
	for i:=0;i<m.NReduce;i++{
		reduceTask:=ReduceTask{id:i}
		for _,mapoutpath:=range m.MapOutPaths{
			for _,path:=range mapoutpath {
				if mapoutpath.Id == i {
					reduceTask.FilePath = append(reduceTask.FilePath,path)
				}
			}
		}
		if !len(reduceTask.FilePath){
			m.ReduceTaskQueue<-reduceTask
			m.ReduceTaskState[i]=false
		}
	}
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
	ret := false
	m.Mut.Lock()
	ret= m.Phase === m.JobOver
	m.Mut.Unlock()
	return ret
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
	m.MapTaskState=make(map[int]bool)
	m.ReduceTaskState=make(map[int]bool)
	m.MapTaskQueue=make(chan MapTask,len(files))
	m.ReduceTaskQueue=make(chan ReduceTask,nReduce)
	m.Phase=MapPhase

	for i,file := range files {
		mapTask:=MapTask{i,file}
		m.MapTaskState[i]=false
		m.MapTaskQueue<-mapTask
	}
	m.server()
	return &m
}
