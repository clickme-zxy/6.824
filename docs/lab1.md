# lab1 MapReduce的完成与实现
## 流程:
1. master 启动，进入服务状态
2. worker向master要求任务，master分状态给予任务：
**master**
* map阶段：给map任务
* reduce阶段：给reduce任务
* 正在运行并且没有新的任务的阶段: holdTask = 0
* 结束运行： job 完成
3. worker执行命令，执行好了之后给master发布信息，要求master更新内容
**worker**
* map任务： 执行map操作
* reduce任务：执行reduce操作
* holdtask=0: 等待，等待之后继续
* 任务结束：worker会退出
**master**
* 更新map任务：
* 标记map任务完成
* 如果到达map任务结束，那么进入shuffle()，开启reduce任务
* 更新reduce任务：
* 标记reduce任务完成
* 如果到达reduce结束，那么任务结束，退出
* 任务结束，退出

## 细节：
1. master需要监控任务是否到达10s,如果到达，那么说明任务没有完成，重新开启任务
2. worker需要监听master是否alive的信号，如果不存在，退出
3. map产生的文件叫做"mr-x-y" reduce产生的文件叫做"mr-out-y"
4. worker崩溃的时候，会出现临时文件，所以一般都是将信息写入临时文件，最后才重命名标准文件，从而避免崩溃产生中间文件
5. 如果master退出之后，worker如何退出，
* master发送一个信号，jobover给worker，worker直到后就会退出
* worker发送心跳，随时监控master的情况

## 不错的链接

我喜欢这个blog的图片
https://blog.csdn.net/u014680339/article/details/105246656
论文中的那张图片也很不错

## 欠缺
Reduce操作中，我直接将所有的文件进行读取，然后操作。实际上是会超过内存的，所以如果采用内部排序+外部排序（mergeSort）
