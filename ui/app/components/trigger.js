import { action } from '@ember/object';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';

const noOp = () => undefined;

export default class Trigger extends Component {
  @tracked error = null;
  @tracked result = null;

  get isBusy() {
    return this.triggerTask.isRunning;
  }

  get isIdle() {
    return this.triggerTask.isIdle;
  }

  get isSuccess() {
    return this.triggerTask.last.isSuccessful;
  }

  get isError() {
    return this.triggerTask.lastErrored;
  }

  get fns() {
    return {
      do: this.onTrigger,
    };
  }

  get onError() {
    return this.args.onError ?? noOp;
  }

  get onSuccess() {
    return this.args.onSuccess ?? noOp;
  }

  get data() {
    const { isBusy, isIdle, isSuccess, isError, result } = this;
    return { isBusy, isIdle, isSuccess, isError, result };
  }

  @task(function*() {
    try {
      this.result = yield this.args.do();
      this.onSuccess(this.result);
    } catch (e) {
      this.error = e;
      this.onError(this.error);
    }
  })
  triggerTask;

  @action
  onTrigger() {
    this.triggerTask.perform();
  }
}
