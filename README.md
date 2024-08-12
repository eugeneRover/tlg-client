## Build tdlib 

[https://tdlib.github.io/td/build.html?language=Go](Instruction)

Replace command with 
`cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX:PATH=../../tdlib ..`

Nevertheless, generated pkg-configs point to wrong dirs. Fix it:
```
cd ~/my-workspace/tdlib
find ./lib/pkgconfig/ -name '*.pc' -exec \
sed -i 's/prefix=\/home\/user\/tdlib/prefix=\/home\/user\/my-workspace\/tdlib/' ./lib/pkgconfig/tdjson.pc {} \;
```

## Build 

`PKG_CONFIG_PATH=/home/user/my-workspace/tdlib/lib/pkgconfig/ go build . `

