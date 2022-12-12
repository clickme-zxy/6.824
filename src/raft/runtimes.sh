#!/bin/bash

rm res -rf
mkdir res

for ((i = 0; i < 10; i++))
do
    (go test -run TestInitialElection2A ) &> ./res/TestInitialElection2A & 
    (go test -run TestReElection2A ) &> ./res/TestReElection2A & 
    (go test -run TestBasicAgree2B ) &> ./res/TestBasicAgree2B & 
    (go test -run TestRPCBytes2B ) &> ./res/TestRPCBytes2B & 
    (go test -run TestFailAgree2B ) &> ./res/TestFailAgree2B & 
    (go test -run TestFailNoAgree2B ) &> ./res/TestFailNoAgree2B & 
    (go test -run TestConcurrentStarts2B ) &> ./res/TestConcurrentStarts2B & 
    (go test -run TestRejoin2B ) &> ./res/TestRejoin2B & 
    (go test -run TestBackup2B ) &> ./res/TestBackup2B & 
    (go test -run TestCount2B ) &> ./res/TestCount2B & 
    
    (go test -run TestPersist12C ) &> ./res/TestPersist12C & 
    (go test -run TestPersist22C ) &> ./res/TestPersist22C & 
    (go test -run TestPersist32C ) &> ./res/TestPersist32C & 
    (go test -run TestFigure82C ) &> ./res/TestFigure82C & 
    (go test -run TestUnreliableAgree2C ) &> ./res/TestUnreliableAgree2C & 
    (go test -run TestFigure8Unreliable2C ) &> ./res/TestFigure8Unreliable2C & 
    (go test -run TestReliableChurn2C ) &> ./res/TestReliableChurn2C & 
    (go test -run TestUnreliableChurn2C ) &> ./res/TestUnreliableChurn2C &   

    sleep 40
    
    grep -nr "FAIL.*raft.*" res

done