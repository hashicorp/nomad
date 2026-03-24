/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { hash } from '@ember/helper';
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

  triggerTask = task(async () => {
    this._reset();
    try {
      this.result = await this.args.do();
      this.onSuccess(this.result);
    } catch (e) {
      this.error = {
        Error: e,
        message: e?.message ?? String(e ?? 'Unknown error'),
      };
      this.onError(this.error);
    }
  });

  onTrigger = () => {
    this.triggerTask.perform();
  };

  <template>{{yield (hash data=this.data fns=this.fns)}}</template>
}
