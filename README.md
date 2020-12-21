# GoSh
GoSh is a simple command execution library for Go.  I'd say its a alpha-quality toy, but I really want to use it in production.

The basis of GoSh is that it can run multi-line blocks of code.  Its like a lousy Shell script interpretter, but better, because its doesn't do all that stuff that's hard to read and understand.  Instead of using operating system environment variables and working directories, GoSh mimics these behaviors.  This allows multiple execution environments with concurrent scripts to be run, with a more deterministic behavior.  Each block has a global error handler, because you never really wanted to handle errors anyway.  Finally, you can trigger Go functions from your blocks of code, because that's the whole point of it all.

GoSh also implements the following Shell-like commands:
 - cd : change the working directory
 - echo : write to the console
 - mkdir : create a directory
 - pushd : change the working directory, remembering the old
 - popd : restore the last remembered directory
 - rm : remove a file
 - rmdir : remove a directory
 - set : save text as a variable

Todo:  write some tests, borrow some https://magefile.org/ magic, somehow support Docker and SSH targets

GoSh is pronounced 'gosh' if you like it, otherwise 'gauche'.
