protoc --proto_path ../../../ -I=./proto --go_out=plugins=grpc:./proto proto/recordbudget.proto
mv proto/github.com/brotherlogic/recordbudget/proto/* ./proto
