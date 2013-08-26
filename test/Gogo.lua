x = 1
print("hello: " .. x+1)

--gogo.include("foo")

CFLAGS = {'-fPIC', '-Wall'}

function gcc()
	return 'gcc'
end

gogo.rule(gcc(), {'main.c'}, {'main.o'})
