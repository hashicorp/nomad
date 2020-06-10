import ApplicationAdapter from './application';

export default class Agent extends ApplicationAdapter {
  pathForType = () => 'agent/members';

  urlForFindRecord() {
    const [, ...args] = arguments;
    return this.urlForFindAll(...args);
  }
}
