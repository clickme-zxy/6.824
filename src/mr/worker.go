package mr

import "fmt"
import "log"
import "net/rpc"
import "hash/fnv"

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
// 传入函数，实现传入mapf 函数和reducef函数
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	wg:=sync.WaitGroup{}
	wg.Add(1)
	go doWork(mapf,reducef,&wg)
	go func() {
		for{
			time.Sleep(5*time.Second)
			ret:=call("master.alive",&args,&reply)
			if !ret {
				return 
			}
		}
		// 为什么这样写呢
		wg.Done()
	}()
	wg.Wait()
}

func doWork(mapf func(string, string) []KeyValue,
reducef func(string, []string) string,wg *sync.WaitGroup){
	defer wg.Done()
	for {
		// 像master申请任务
		reply:= GetTask()
		if !replyMsg.HoldTask() {
			time.Sleep(time.Second)
			continue
		}
		switch reply.Phase {
			case MapPhase:
				opaths,ok:=doMap(&reply,mapf)
				if ok==true {
					ret:=call("Master.Update",&RPCArgs{MapPhase,reply.MapTask.Id,opaths},&reply)
					if ret{
						// fmt.Println("map:%v success",reply.Id)
					}else{
						// fmt.Println("map:%v fail",reply.Id)
					}
				}else{
					// fmt.Println("map:%v fail",reply.Id)
				}
			case ReducePhase:
				opath,ok:=doReduce(&reply,mapf)
				if ok==true {
					ret:=call("Master.Update",&RPCArgs{MapPhase,reply.MapTask.Id,opath},&reply)
					if ret{
						// fmt.Println("reduce:%v success",reply.Id)
					}else{
						// fmt.Println("reduce:%v fail",reply.Id)
					}
				}else{
					// fmt.Println("reduce:%v fail",reply.Id)
				}
			case TaskWait:
				time.Sleep(time.Second)
			case JobOver:
				break
		}
	}
}

// Id int	// 编号
// Phase State	// 状态
// OutPaths []Response // 输出路径
func doMap(reply *RPCReply,mapf func(string, string) []KeyValue) {

}

func doMap(reply *RPCReply,reducef func(string, []string) string) string{
	
}

//
// example function to show how to make an RPC call to the master.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	args := RPCArgs{}
	reply := RPCReply{}
	// send the RPC request, wait for the reply.
	call("Master.rpc", &args, &reply)
}

//
// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
