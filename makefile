TARGET = smock.exe

all: $(TARGET)

$(TARGET):
	go build  -o $(TARGET) -ldflags " \
	-X github.com/zddava/smock/build.Module=SMOCK \
	-X github.com/zddava/smock/build.Version=0.0.1 \
	-X github.com/zddava/smock/build.Date=20231008"

clean:
	rm -f $(TARGET) *.o
