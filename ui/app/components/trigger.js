/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { action } from '@ember/object';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';
import { schedule } from '@ember/runloop';

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
    return this.triggerTask.last?.isSuccessful;
  }

  get isError() {
    return !!this.error;
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

  _reset() {
    this.result = null;
    this.error = null;
  }

  @task(function* () {
    this._reset();
    try {
      this.result = yield this.args.do();
      this.onSuccess(this.result);
    } catch (e) {
      this.error = { Error: e };
      this.onError(this.error);
    }
  })
  triggerTask;

  @action
  onTrigger() {
    schedule('actions', () => {
      this.triggerTask.perform();
    });
  }
}
