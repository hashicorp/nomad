import Service from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { schedule } from '@ember/runloop';
import { getBoxToBoxArrow } from 'curved-arrows';

function boxToArrow(ra, rb) {
  const bbA = ra.getBoundingClientRect();
  const bbB = rb.getBoundingClientRect();

  const [sx, sy, c1x, c1y, c2x, c2y, ex, ey, ae, as] = getBoxToBoxArrow(
    bbA.x,
    bbA.y,
    bbA.width,
    bbA.height,
    bbB.x,
    bbB.y,
    bbA.width,
    bbB.height
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
      .filter((e) => e.data.prevEval) // should we make this more generic
      .map((e) => {
        const {
          data: { prevEval: pid, id },
        } = e;

        const eRectangle = document.querySelector(`[data-eval="${id}"]`);
        const prevRectangle = document.querySelector(`[data-eval="${pid}"]`);

        return [eRectangle, prevRectangle];
      });
    // find previousEval if it exists calculate and add add a path
  }

  get relationships() {
    const { rects } = this;

    return rects.map(([eRectangle, prevRectangle]) => {
      const { sx, sy, c1x, c1y, c2x, c2y, ex, ey } = boxToArrow(
        eRectangle,
        prevRectangle
      );

      return `M ${sx} ${sy} C ${c1x} ${c1y}, ${c2x} ${c2y}, ${ex} ${ey}`;
    });
  }

  @action
  recalcCurves() {
    // retrigger the tracked getters by resetting dependent keys
    this.actors = this.actors;
  }
}
