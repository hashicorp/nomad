import Component from '@ember/component';

export default Component.extend({
  mode1: 'stdout',
  isPlaying1: true,

  sampleOutput: `Sample output
> 1
> 2
> 3
[00:12:58] Log output here
[00:15:29] [ERR] Uh oh
Loading.
Loading..
Loading...

  >> Done! <<

`,

  sampleError: `Sample error

[====|--------------------] 20%

!!! Unrecoverable error:

  Cannot continue beyond this point. Exception should be caught.
  This is not a mistake. You did something wrong. Check the code.
  No, you will not receive any more details or guidance from this
  error message.

`,
});
