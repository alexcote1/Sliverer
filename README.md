# Sliverer
runs sliver command on all hosts, partially (or for run on new basically entirely) based on example in sliver repo


to install 
```
go get github.com/alexcote1/Sliverer
```
to run 
```
Sliverer pwnboard --url="https://192.2.2.2" 
```
or 
```
Sliverer command --command="bash" --args="-c^ ls" 
```
if you need to specify a config specify its path with 
```
--config="/home/yourname/.sliver-client/configs/configname.txt"
```


