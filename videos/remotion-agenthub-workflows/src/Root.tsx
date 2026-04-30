import {Composition} from 'remotion';
import {AgentHubWorkflows} from './AgentHubWorkflows';

export const Root = () => {
  return (
    <Composition
      id="AgentHubWorkflows"
      component={AgentHubWorkflows}
      durationInFrames={16 * 30}
      fps={30}
      width={1920}
      height={1080}
    />
  );
};
