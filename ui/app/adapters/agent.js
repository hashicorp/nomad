import ApplicationAdapter from './application';

export default class AgentAdapter extends ApplicationAdapter {
  pathForType = () => 'agent/members';

  urlForFindRecord() {
    const [, ...args] = arguments;
    return this.urlForFindAll(...args);
  }
}
