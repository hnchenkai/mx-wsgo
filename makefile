clean:
	rm -rf bytecoder/js
go:
	cd bytecoder && buf generate
js:
	rm -rf bytecoder/js
	mkdir bytecoder/js
	pbjs -t static-module -o bytecoder/js/message.pb.js bytecoder/message.proto
	pbts -o bytecoder/js/message.pb.d.ts bytecoder/js/message.pb.js