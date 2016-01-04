#!/bin/bash

platforms=( darwin linux freebsd windows openbsd )

programs=( sparkyfish-cli sparkyfish-server )

for prog in "${programs[@]}"
do
  cd ${prog}
  echo "--> Building ${prog}"
  for plat in "${platforms[@]}"
  do 
    echo "----> Building for ${plat}/amd64"
    if [ "$plat" = "windows" ]; then
      GOOS=$plat GOARCH=amd64 go build -o ${prog}-win64.exe
      echo "Compressing..."
      zip -9 ${prog}-win64.zip ${prog}-win64.exe
      mv ${prog}-win64.zip ../binaries/${prog}/
      rm ${prog}-win64.exe
    else
       OUT="${prog}-${plat}-amd64"
       GOOS=$plat GOARCH=amd64 go build -o $OUT
       echo "Compressing..."
       gzip -f $OUT
       mv ${OUT}.gz ../binaries/${prog}/
    fi 
  done
  cd ..
done
