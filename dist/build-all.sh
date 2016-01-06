#!/bin/bash

TAG=`git describe --tags`

platforms=( darwin linux freebsd windows openbsd )

programs=( sparkyfish-cli sparkyfish-server )

for prog in "${programs[@]}"
do
  PROG_WITH_TAG=${prog}-${TAG}
  cd ${prog}
  echo "--> Building ${prog}"
  for plat in "${platforms[@]}"
  do
    echo "----> Building for ${plat}/amd64"
    if [ "$plat" = "windows" ]; then
      GOOS=$plat GOARCH=amd64 go build -o ${PROG_WITH_TAG}-win64.exe
      echo "Compressing..."
      zip -9 ${PROG_WITH_TAG}-win64.zip ${PROG_WITH_TAG}-win64.exe
      mv ${PROG_WITH_TAG}-win64.zip ../binaries/${prog}/
      rm ${PROG_WITH_TAG}-win64.exe
    else
       OUT="${PROG_WITH_TAG}-${plat}-amd64"
       GOOS=$plat GOARCH=amd64 go build -o $OUT
       echo "Compressing..."
       gzip -f $OUT
       mv ${OUT}.gz ../binaries/${prog}/
    fi
  done

  # Build Linux/ARM
  echo "----> Building for linux/arm"
  OUT="${PROG_WITH_TAG}-linux-arm"
  GOOS=linux GOARCH=arm go build -o $OUT
  echo "Compressing..."
  gzip -f $OUT
  mv ${OUT}.gz ../binaries/${prog}/
  cd ..
done
