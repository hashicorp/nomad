// @ts-check
import { default as ApplicationAdapter, namespace } from './application';
import ActionModel from '../models/action';

export default class ActionAdapter extends ApplicationAdapter {
  /**
   * @param {ActionModel} action
   * @returns
  */
  perform(action) {
    const { name, args, command } = action;
    console.log('action adapter perform', args, action.job);
    const url = `/${this.namespace}/client/allocation/${action.job.get('allocations').objectAt(0).id}/exec`;
    console.log('url', url);
    return this.ajax(url, 'POST', {
      data: {
        name,
        args: JSON.stringify(args),
        command        
      },
    });
  }
  
}
