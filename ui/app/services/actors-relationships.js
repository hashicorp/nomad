/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { schedule } from '@ember/runloop';
import { getBoxToBoxArrow } from 'curved-arrows';

function boxToArrow(ra, rb) {
  const bbA = ra;
  const bbB = rb;

  const [sx, sy, c1x, c1y, c2x, c2y, ex, ey, ae, as] = getBoxToBoxArrow(
    bbA.offsetLeft,
    bbA.offsetTop,
    bbA.offsetWidth,
    bbA.offsetHeight,
    bbB.offsetLeft,
    bbB.offsetTop,
    bbB.offsetWidth,
    bbB.offsetHeight
  );

  return {
    sx,
    sy,
    c1x,
    c1y,
    c2x,
    c2y,
    ex,
    ey,
    ae,
    as,
  };
}

export default class ActorRelationshipService extends Service {
  @tracked actors = [];

  get fns() {
    const { registerActor, deregisterActor, recalcCurves } = this;
    return { registerActor, deregisterActor, recalcCurves };
  }

  get data() {
    const { actors, relationships } = this;
    return { actors, relationships };
  }

  @action registerActor(actor) {
    schedule('actions', this, () => {
      this.actors = [...this.actors, actor];
    });
  }

  @action deregisterActor(actor) {
    schedule('actions', this, () => {
      this.actors = this.actors.filter((a) => a !== actor);
    });
  }

  get rects() {
    const { actors } = this;

    return actors
      .filter((e) => e.previousEval)
      .map((e) => {
        const { previousEval: pid, id } = e;

        const eRectangle = document.querySelector(`[data-eval="${id}"]`);
        const prevRectangle = document.querySelector(`[data-eval="${pid}"]`);

        return [eRectangle, prevRectangle];
      });
  }

  get relationships() {
    const { rects } = this;

    return rects.map(([eRectangle, prevRectangle]) => {
      const { sx, sy, c1x, c1y, c2x, c2y, ex, ey } = boxToArrow(
        eRectangle,
        prevRectangle
      );

      return {
        d: `M ${sx} ${sy} C ${c1x} ${c1y}, ${c2x} ${c2y}, ${ex} ${ey}`,
        sx,
        sy,
        ex,
        ey,
      };
    });
  }

  @action
  recalcCurves() {
    // retrigger the tracked getters by resetting dependent keys
    /* eslint-disable-next-line */
    this.actors = this.actors;
  }
}
