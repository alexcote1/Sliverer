# Sliverer
runs sliver command on all hosts, partially based on example in sliver repo


to install 
```
go get github.com/alexcote1/Sliverer
```
to run 
```
Sliverer --command="ls" --runonnew=true
```
or 
```
Sliverer --command="ls" 
```
if you need to specify a config specify its path with 
```
--config="/home/yourname/.sliver-client/configs/configname.txt"
```


