import Model from '@ember-data/model';
import { attr, belongsTo } from '@ember-data/model';

export default class ActionInstanceModel extends Model {
  @belongsTo('action') action;

  /**
   * @type {'starting'|'running'|'complete'}
   */
  @attr('string') state;

  @attr('string', {
    defaultValue() {
      return '';
    },
  })
  messages;
  @attr('date', {
    defaultValue() {
      return new Date();
    },
  })
  createdAt;

  @attr('date', {
    defaultValue() {
      return null;
    },
  })
  completedAt;

  @attr('string') allocID;

  // stop() {
  //   console.log('ok stopping instance lol');
  //   console.log('thissocket', this.socket,)
  //   this.socket.stop();
  // }

  /**
   * @type {WebSocket}
   */
  @attr() socket;
}
