package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

type ByKey []KeyValue

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

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
// 优秀！
// 给workder写了个心跳，两个函数中哪一个先返回，程序都结束~~~
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	// ticky: this is excellent
	go doWork(mapf, reducef, &wg)
	go func() {
		for {
			time.Sleep(time.Second)
			args := RPCArgs{}
			reply := RPCReply{}
			ret := call("Master.Alive", &args, &reply)
			if !ret || reply.Phase == JobOver {
				break
			}
		}
		wg.Done()
	}()
	wg.Wait()
}

func doWork(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string, wg *sync.WaitGroup) {
	defer wg.Done()
	// work until end
	for {
		// we assume that it is just a network error
		reply, ok := getTask()
		if ok == false || reply.HoldTask == false {
			time.Sleep(time.Second)
			continue
		}
		switch reply.Phase {
		case MapPhase:
			opaths, ok := doMap(&reply, mapf)
			if ok == true {
				ret := call("Master.Update", &RPCArgs{reply.MapTask.Id, MapPhase, opaths}, &reply)
				if ret {
					// fmt.Printf("map:%v success\n", reply.MapTask.Id)
				} else {
					// fmt.Printf("map:%v fail\n", reply.MapTask.Id)
				}
			} else {
				// fmt.Printf("map:%v fail\n", reply.MapTask.Id)
			}
		case ReducePhase:
			opath, ok := doReduce(&reply, reducef)
			if ok == true {
				ret := call("Master.Update", &RPCArgs{reply.ReduceTask.Id, ReducePhase, opath}, &reply)
				if ret {
					// fmt.Printf("reduce:%v success\n", reply.ReduceTask.Id)
				} else {
					// fmt.Printf("reduce:%v fail\n", reply.ReduceTask.Id)
				}
			} else {
				//fmt.Printf("reduce:%v fail\n", reply.ReduceTask.Id)
			}
		case TaskWait:
			time.Sleep(time.Second)
		case JobOver:
			return
		}
	}
}

func doMap(reply *RPCReply, mapf func(string, string) []KeyValue) ([]Response, bool) {
	mapTask := reply.MapTask
	NReduce := reply.Nreduce
	filename := mapTask.FilePath
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	file.Close()

	intermediate := mapf(filename, string(content))
	res := make([][]KeyValue, NReduce)
	x := mapTask.Id
	outPaths := make([]Response, 0)

	for _, kv := range intermediate {
		i := ihash(kv.Key) % NReduce
		res[i] = append(res[i], kv)
	}

	for i := 0; i < NReduce; i++ {
		if len(res[i]) != 0 {
			oname := "mr-" + strconv.Itoa(x) + "-" + strconv.Itoa(i)
			ofile, _ := os.Create(oname)
			enc := json.NewEncoder(ofile)
			for _, kv := range res[i] {
				e := enc.Encode(kv)
				if e != nil {
					fmt.Printf("Map failed: %v, write error\n", x)
				}
			}
			ofile.Close()
			response := Response{i, oname}
			outPaths = append(outPaths, response)
		}
	}
	return outPaths, true
}

//
// call Reduce on each distinct key in intermediate[],
// and print the result to mr-out-0.
//
/*
	为了防止crash在这里用了临时文件来进行实现
*/
func doReduce(reply *RPCReply, reducef func(string, []string) string) ([]Response, bool) {
	reduceTask := reply.ReduceTask
	intermediate := []KeyValue{}
	id := reduceTask.Id
	var outPaths []Response
	for _, filename := range reduceTask.FilePaths {
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}
		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			intermediate = append(intermediate, kv)
		}
		if file != nil {
			os.Remove(file.Name())
		}
	}
	sort.Sort(ByKey(intermediate))
	oname := "mr-out-" + strconv.Itoa(reply.ReduceTask.Id)
	ofile, err := ioutil.TempFile("./", "reduce-out-tmp"+strconv.Itoa(reply.ReduceTask.Id))

	if err != nil {
		fmt.Printf("Reduce failed: %v\n,can not create file", reply.ReduceTask.Id)
		return outPaths, false
	}

	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		var values []string
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)
		fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)
		i = j
	}
	ofile.Close()
	os.Rename(ofile.Name(), oname)
	outPaths = append(outPaths, Response{id, oname})
	return outPaths, true
}

func getTask() (RPCReply, bool) {
	args := RPCArgs{}
	reply := RPCReply{}
	ret := call("Master.GetTask", &args, &reply)
	if !ret {
		return reply, false
	}
	return reply, true
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
