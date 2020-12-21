# GoSh
GoSh is a simple command execution library for Go.  I'd say its a alpha-quality toy, but I really want to use it in production.

The basis of GoSh is that it can run a multi-line blocks of code.  Its like a lousy Shell script interpretter because its simple and hard to abuse.  Instead of using operating system environment variables and working directories, GoSh mimics these behaviors.  This allows multiple execution environments to be concurrent scripts to be run, with more deterministic behavior.  Each block has a global error handler, because you never really wanted to handle errors anyway.  Finally, you can trigger Go functions from your blocks of code, because that's the whole point of it all.

GoSh also implements the following Shell-like commands:
 - cd : change the working directory
 - echo : write to the console
 - mkdir : create a directory
 - pushd : change the working directory, remembering the old
 - popd : restore the last remembered directory
 - rm : remove a file
 - rmdir : remove a directory
 - set : save text as a variable

GoSh is pronounced 'gosh' if you like it, otherwise 'gauche'.
