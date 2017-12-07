import Ember from 'ember';
import wait from 'ember-test-helpers/wait';

const { assign, run } = Ember;

const waitActions = ['|', '.'];
const ecActions = {
  '.': () => {},
  '|': () => run.cancelTimers(),
};

export default function ecWire(wire, interval, userActions) {
  const steps = wire.split('');
  const actions = assign({}, userActions, ecActions);

  const sequences = [];
  while (steps.length) {
    sequences.push(constructSequence(steps));
  }

  let returnPromise;
  sequences.forEach(sequence => {
    const executor = translateSequence(sequence, interval, actions);
    if (returnPromise) {
      returnPromise = returnPromise.then(wait).then(executor);
    } else {
      executor();
      returnPromise = wait();
    }
  });
  return returnPromise;
}

function translateSequence(sequence, interval, actions) {
  return () => {
    run.later(() => {
      sequence.actions.forEach(action => {
        if (typeof action === 'string') {
          if (!actions[action]) {
            throw new Error(`Unexpected action "${action}" in EC Wire`);
          }
          actions[action]();
        } else {
          translateSequence(action, interval, actions)();
        }
      });
    }, sequence.time * interval);
  };
}

function constructSequence(steps) {
  const batch = [];

  // Collect steps
  while (steps.length) {
    const step = steps.shift();
    batch.push(step);
    if (waitActions.includes(step)) {
      break;
    }
  }

  return constructFrame(batch);
}

function constructFrame(steps) {
  const waitFrames = [];
  const actions = [];

  while (steps.length && steps[0] === '-') {
    waitFrames.push(steps.shift());
  }

  while (steps.length && steps[0] !== '-') {
    actions.push(steps.shift());
  }

  if (steps.length) {
    actions.push(constructFrame(steps));
  }

  return {
    time: waitFrames.length,
    actions: actions,
  };
}
