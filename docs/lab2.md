# 6.824 lab2 raft的解析

## 一、分析

* 三个状态

1. candidate
2. follower
3. leader

* 两个RPC:

1. requestVoteRpc
2. appendentriesRPc

* 三个循环：

1. heartbeat循环
2. election循环
3. apply循环（也可以采用condition，收到信息后，通知全部的节点）

### 流程

参照raft相关的图片，具体的raft的流程如下

一开始状态时follow、然后随着election定时器到达时间，转化为了candidate，

达到candidate之后，分为以下三种情况：

> 1. 超出定时器还没有选举成功，转化为candidate
> 2. 选举成功，获得半数选票，变成leader
> 3. 收到更高的任期，变成follow

之后如果是leader的话，需要定期发送心跳，也就是heartbeat循环，

如果是candidate 和 follow的话，就需要election循环，来看自己是否与leader保持联系

所有的node，都需要apply,来定期的检查，是否需要更新我的log应用到机器上。

### 三个状态的转换函数

从上面得知，我们需要三个函数，

* become follower

> state = follower
>
> term = 最新的任期
>
> votedfor = -1(更新任期的时候，没有投票，设为初始值，-1)

作为follower,需要能够回应requestRPC和appendRPC这两个RPC

* become candidate

> state = candidate
>
> term +=1 term会自动加1
>
> votedfor = candidateId 为自己投票

并且需要发送requestVoteRPC给所有的对等节点,同时处理得到的reply,并且响应其他的RPC 

* become leader

> state = leader

需要给所有的节点发送心跳，告知我是leader

### 三个循环

* election循环（对于follower和election），本质是一个定时器，在规定的时间内，如果收到信号，那么就算成功

下面是说**重置的时间**

> candidate 
>
> 收到更高的任期，那么我会重置我的electionTimer，并且变成follower
>
> 如果变成leader,就不存在electionTimer这个事情

> follower
>
> 如果收到leader的appendEntries，并且这个appendEntries确实是当前的leader发的，而不是过期的（这个地方很容易出现bug~~~）,那么重置我的心跳
>
> 如果收到candidate的requestVote,并且candidate确实可以被投票，那么重置心跳

在关于requestvote中会讲解如果超时，那么会怎么做

* heartbeat循环(对于leader)

> leader
>
> 定时sleep,醒了之后发送心跳，在这里我直接每个心跳阶段发送需要的信息，而不是在start收到信息之后，用condition的方式通知代价处理信息。

* apply（对于所有的节点）

> 定时之后，查询commitId 和 lastapplied是否一致，不一致的话,将lastapplied更新到commit阶段

### 两个RPC

* requestVotedRPC

这个是candidate想成为leader,所以发起的投票，candidate自己会投自己一票，所以只需要发送给其他的节点就可以了,

当其他节点接收到这个RPC处理的流程是

> 1. args.Term<rf.Term(比我的任期低)，拒绝投票,并且返回我的任期
> 2. args.Term>rf.Term 那么我成为了followers,并且重新设置定时器，此时我的votefor = -1
>    * 检查args.lastlog和rf.lastlog 我们之间谁最新，如果是我，那么拒绝投票
>    * 否则，进行投票
> 3. args.term == rf.Term ,那么
>    * 检查我在当前任期是否投票，以及我投票的对象是否是candidateId，如果不满足条件，那么说明我已经在当前任期给非candidateId的信息投过票，那么我需要拒绝投票
>    * 如果没有投个票，那么投个这个节点，并且重置时间计时器

当candidate收到信息的处理:

> 1. reply.Term>rf.Term ,收到比我大的任期，变成follower,重置时间计数器
> 2. reply.Term<rf.Term,收到过期的信息，返回，不考虑处理
> 3. reply.Term == rf.Term ，那么收到信息，进行处理
>    * 同意投票，计数+1，如果超过一半成为leader
>    * 反对投票

#### 注意点：

1. 我需要检查**我是否给candidate投过票，如果投过票，那么我的节点返回的是同意投票**，并且重置时间定时器
2. requestVoteRPC的reply的处理中，需要小心**收到过期的投票信息**，这个是非常容易错误的点

* appendEntriesRPC

这个地方首先讲解一下简单操作（太慢了，容易卡测试），以及一个回滚的日志操作

leader从heartbeat中定时的其他节点发送信息，具体的内容是参考它的nextIndex来进行发送的

> leader 发送信息:
>
> 找到节点的nextIndex，根据nextIndex，发送相关的logs 以及preoVlog给节点

> 节点接收信息：
>
> 节点首先判断日志的情况（上面有节点，这边不详细讲了）
>
> 然后比较prevLoG和我的lastLog之间的关系
>
>  * prevLog > lastlog 那么返回false
>  * prevLog < =lastlog ，判断节点任期是否一致，如果任期一致，那么说明prevLog之前的全部一致，之后的进行覆盖

> leader 接收到返回的信息:
>
> * 如果任期比leader大 ，leader变成follower
> * 如果任期比leader小，说明过期，直接不处理
> * 如果任期一致，那么说明需要进行处理
>   * success 成功，更新对应的matchIndex 和nextIndex
>   * fail 失败， nextIndex--，之后会再次尝试（我设置了心跳定时，所以在心跳阶段会进行更新）
> * leader由于更新了matchIndex,如果matchIndex收到一半以上（也就是排序后的中位数），那么说明这个信息有一半都被执行，就可以更新commitIndex这个信号

nextIndex--这个操作太慢了，会造成失败，实际上可以尝试，一次回退到一个前一个任期，老师的讲解具体的失败后的任期的更新如下:

>We believe the protocol the authors probably want you to follow is:
>
>- If a follower does not have prevLogIndex in its log, it should return with conflictIndex = len(log) and conflictTerm = None.
>- If a follower does have prevLogIndex in its log, but the term does not match, it should return conflictTerm = log[prevLogIndex].Term, and then search its log for the first index whose entry has term equal to conflictTerm.
>- Upon receiving a conflict response, the leader should first search its log for conflictTerm. If it finds an entry in its log with that term, it should set nextIndex to be the one beyond the index of the last entry in that term in its log.
>- If it does not find an entry with that term, it should set nextIndex = conflictIndex.

> 回退的原则如下：
>
> 1. 如果follower不存在这个prevLogIndex在它的log里面，那么说明太小了，我的confilictIndex就变成log的长度，term变成null
> 2. 如果follow拥有prevLogIndex在它的日志里面，但是不匹配，那么认为当前日志冲突，找到第一个日志和confilctTerm一样的，那就是需要更换的地方
> 3. 如果leader收到一个冲突的信息，首先先找到当前currentTerm的一条log,如果找到了，那么nextIndex，那么我需要找到这个日志的下一个任期的第一个log,作为更替
> 4. 如果找不到的话，那么我的nextIndex = conflictIndex

### 数据结构

* raft的结构

> 1. currentTerm 当前任期
> 2. votedfor 在当前的任期是否投过票（需要持久化，从而避免重复投票）
> 3. log[] 日志条目，每个条目包含状态机以及需要执行的命令和领导人收到的任期号

上面的信息都需要持久的存储

> 1. commitIndex
> 2. lastapplied  被状态机执行的最大日志条目的索引值

除此之外，作为领导人需要的信息

> 1. nextIndex 继续需要发给服务器的下一个索引
> 2. matchIndex 对于每个服务器，复制的最高索引值

因此，我们可以推出raft的基本信息是

我们还需要一个额外的信息

> 1. state 表明当前的状态
> 2. 以及electionTimer 选举时间，用来重置，但是一般也可以用channel来实现

附上我看到用channel实现，而且感觉很不错的例子

* log的结构
* appendentries的结构
* requestvote的结构......

## 持久化

最后别忘了persisit()

readpersist要记得加锁，我之前忘记了，导致了错误~

