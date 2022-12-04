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
		reply:= doGetTask()
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
// type Response struct{
// 	Id int // 编号
// 	OutPath string //输出的路径
// }

// type RPCReply struct {
// 	Phase State	// 状态
// 	HoldTask bool // 是否持有任务
// 	Nreduce int 
// 	MapTask MapTask
// 	ReduceTask MapTask
// }

// type MapTask struct {
// 	Id int	// 编号
// 	FilePath string  // 文件
// }
func doMap(reply *RPCReply,mapf func(string, string) []KeyValue) {
	// 进行分割，并且放在对应的文件中
	intermediate := []mr.KeyValue{}
	mapTask:= reply.mapTask
	NReduce :=reply.NReduce
	filename:=mapTask.FilePath
	file,err:=os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	file.Close()
	// 进行了分割的操作
	// 接下来需要进行排序和映射
	kva := mapf(filename, string(content))
	intermediate = append(intermediate, kva...)
	sort.Sort(ByKey(intermediate))
	x:=mapTask.Id
	outPaths :=make([]Response)
	// 排序后逐个写入
	for i<len(intermediate) {
		j:=i+1
		for j<len(intermediate)&&intermediate[j].Key == intermediate[i].Key {
			j++
		}
		y:=ihash(intermediate[i].Key)
		oname:=fmt.Sprintf("mr-%v-%v",x,y)
		ofile, _ := os.Create(oname)
		response := Response{y,oname}
		outPaths = append(outPaths,response)
		for k:=i;k<j;k++ {
			fmt.Fprintf(ofile, "%v %v\n", intermediate[k].Key, intermediate[k].Value)
		}
		i=j
		ofile.Close()
	}	
}

// type ReduceTask struct {
// 	FilePaths []string //文件
// 	Id int	//编号
// }
func doReduce(reply *RPCReply,reducef func(string, []string) string) string{
	reduceTask:=reply.ReduceTask
	intermediate := []mr.KeyValue{}
	id:=reduceTask.Id
	//
	// call Reduce on each distinct key in intermediate[],
	// and print the result to mr-out-0.
	//
	for _,filename:=range reduceTask.FilePaths{
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}
		content, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatalf("cannot read %v", filename)
		}
		file.Close()
		kva := mapf(filename, string(content))
		intermediate = append(intermediate, kva...)

		oname :=fmt.Sprintf("mr-out-%v", id)
		ofile, _ := os.Create(oname)
		i := 0
		for i < len(intermediate) {
			j := i + 1
			for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
				j++
			}
			values := []string{}
			for k := i; k < j; k++ {
				values = append(values, intermediate[k].Value)
			}
			output := reducef(intermediate[i].Key, values)
			fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)
			i = j
		}
		ofile.Close()
	}
}

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
