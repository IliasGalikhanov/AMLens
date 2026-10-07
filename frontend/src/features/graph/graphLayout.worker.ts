import { createLayoutTask, type LayoutCommand, type LayoutFrame } from './graphLayoutTask';

let task: ReturnType<typeof createLayoutTask> | undefined;
self.onmessage = (event: MessageEvent<LayoutCommand>) => {
  const message = event.data;
  if (message.type === 'init') {
    task?.dispose();
    task = createLayoutTask(message.input, (frame: LayoutFrame) => {
      self.postMessage(frame, { transfer: [frame.positions.buffer] });
    });
  } else if (message.type === 'configure') task?.configure(message.forces, message.motion);
  else task?.pin(message.id, message.point);
};
