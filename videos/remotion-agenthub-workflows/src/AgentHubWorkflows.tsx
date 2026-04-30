import React from 'react';
import {
  AbsoluteFill,
  Easing,
  Sequence,
  interpolate,
  spring,
  useCurrentFrame,
  useVideoConfig,
} from 'remotion';

const palette = {
  bg: '#060814',
  bg2: '#0a0f24',
  fg: '#d6f5ff',
  dim: '#6b7aa8',
  cyan: '#5ef0ff',
  magenta: '#ff4ed6',
  lime: '#a3ff4e',
  orange: '#ff9f43',
};

const gridStyle: React.CSSProperties = {
  position: 'absolute',
  inset: 0,
  backgroundImage:
    'linear-gradient(90deg, rgba(120,255,220,0.12) 1px, transparent 1px), linear-gradient(0deg, rgba(120,255,220,0.12) 1px, transparent 1px)',
  backgroundSize: '72px 72px',
  opacity: 0.35,
};

const panelStyle: React.CSSProperties = {
  width: '100%',
  height: '100%',
  padding: '64px 72px',
  background: 'rgba(10,15,36,0.68)',
  border: '1px solid rgba(120,255,220,0.2)',
  boxShadow: '0 24px 72px rgba(0,0,0,0.35)',
  clipPath:
    'polygon(24px 0, calc(100% - 4px) 0, 100% 4px, 100% calc(100% - 24px), calc(100% - 24px) 100%, 4px 100%, 0 calc(100% - 4px), 0 24px)',
  position: 'relative',
  overflow: 'hidden',
};

const SceneShell: React.FC<{children: React.ReactNode}> = ({children}) => {
  return (
    <AbsoluteFill
      style={{
        padding: 110,
        color: palette.fg,
        fontFamily: 'Inter, Segoe UI, system-ui, sans-serif',
      }}
    >
      <div style={panelStyle}>{children}</div>
    </AbsoluteFill>
  );
};

const Eyebrow: React.FC<{children: React.ReactNode}> = ({children}) => (
  <div
    style={{
      color: palette.cyan,
      fontSize: 24,
      letterSpacing: '0.32em',
      textTransform: 'uppercase',
      fontWeight: 700,
    }}
  >
    {children}
  </div>
);

const Title: React.FC<{children: React.ReactNode; size?: number}> = ({
  children,
  size = 96,
}) => (
  <div
    style={{
      fontSize: size,
      lineHeight: 0.92,
      fontWeight: 800,
      marginTop: 18,
      maxWidth: 1280,
    }}
  >
    {children}
  </div>
);

const Subtitle: React.FC<{children: React.ReactNode; style?: React.CSSProperties}> = ({
  children,
  style,
}) => (
  <div
    style={{
      marginTop: 24,
      fontSize: 34,
      color: palette.dim,
      lineHeight: 1.4,
      maxWidth: 1100,
      ...style,
    }}
  >
    {children}
  </div>
);

const FooterLine: React.FC<{left: string; right: string; rightAccent?: boolean}> = ({
  left,
  right,
  rightAccent,
}) => (
  <div
    style={{
      position: 'absolute',
      left: 72,
      right: 72,
      bottom: 54,
      display: 'flex',
      justifyContent: 'space-between',
      color: palette.dim,
      fontSize: 24,
      textTransform: 'uppercase',
      letterSpacing: '0.12em',
    }}
  >
    <span>{left}</span>
    <span style={rightAccent ? {color: palette.lime, fontWeight: 700} : undefined}>{right}</span>
  </div>
);

const Card: React.FC<{
  step: string;
  label: string;
  copy?: string;
  style?: React.CSSProperties;
}> = ({step, label, copy, style}) => (
  <div
    style={{
      flex: 1,
      minHeight: 238,
      padding: 28,
      background: 'linear-gradient(180deg, rgba(12,18,40,0.92), rgba(7,11,28,0.88))',
      border: '1px solid rgba(120,255,220,0.16)',
      clipPath:
        'polygon(16px 0, calc(100% - 3px) 0, 100% 3px, 100% calc(100% - 16px), calc(100% - 16px) 100%, 3px 100%, 0 calc(100% - 3px), 0 16px)',
      display: 'flex',
      flexDirection: 'column',
      justifyContent: 'space-between',
      boxShadow: '0 16px 42px rgba(0,0,0,0.24)',
      ...style,
    }}
  >
    <div
      style={{
        fontSize: 18,
        letterSpacing: '0.18em',
        textTransform: 'uppercase',
        color: palette.orange,
        fontWeight: 700,
      }}
    >
      {step}
    </div>
    <div>
      <div style={{fontSize: 44, lineHeight: 1.04, fontWeight: 750}}>{label}</div>
      {copy ? (
        <div style={{fontSize: 24, lineHeight: 1.35, marginTop: 12, color: palette.dim}}>
          {copy}
        </div>
      ) : null}
    </div>
  </div>
);

const AnimatedBlock: React.FC<{
  delay: number;
  children: React.ReactNode;
  fromY?: number;
  fromX?: number;
}> = ({delay, children, fromY = 32, fromX = 0}) => {
  const frame = useCurrentFrame();
  const {fps} = useVideoConfig();
  const progress = spring({
    fps,
    frame: frame - delay,
    config: {damping: 200, stiffness: 180},
  });
  return (
    <div
      style={{
        opacity: progress,
        transform: `translate(${interpolate(progress, [0, 1], [fromX, 0])}px, ${interpolate(progress, [0, 1], [fromY, 0])}px)`,
      }}
    >
      {children}
    </div>
  );
};

const SceneOne = () => (
  <SceneShell>
    <AnimatedBlock delay={4} fromY={24}>
      <Eyebrow>agenthub · workflow engine</Eyebrow>
    </AnimatedBlock>
    <AnimatedBlock delay={8} fromY={50}>
      <Title>
        AgentHub <span style={{color: palette.magenta}}>Workflows</span>
      </Title>
    </AnimatedBlock>
    <AnimatedBlock delay={18} fromY={32}>
      <Subtitle>
        Del mensaje al sistema, sin cambiar de superficie. Chat, contexto, tools y
        operaciones seguras en un solo flujo.
      </Subtitle>
    </AnimatedBlock>
    <AnimatedBlock delay={28} fromY={18}>
      <FooterLine left="web + whatsapp + proyectos" right="16s explainer" rightAccent />
    </AnimatedBlock>
  </SceneShell>
);

const SceneTwo = () => (
  <SceneShell>
    <AnimatedBlock delay={4} fromY={24}>
      <Eyebrow>1 · chat operativo</Eyebrow>
    </AnimatedBlock>
    <AnimatedBlock delay={8} fromY={40}>
      <Title size={74}>De la consulta al tool call</Title>
    </AnimatedBlock>
    <div style={{display: 'flex', gap: 24, marginTop: 56}}>
      <AnimatedBlock delay={16} fromX={-70} fromY={0}>
        <Card step="entrada" label="Web / WhatsApp" copy="Un único chat como superficie de trabajo." />
      </AnimatedBlock>
      <AnimatedBlock delay={22} fromY={80}>
        <Card step="contexto" label="Memoria y contexto" copy="Contexto persistente antes de ejecutar." />
      </AnimatedBlock>
      <AnimatedBlock delay={28} fromX={70} fromY={0}>
        <Card step="acción" label="Tools y respuesta" copy="Sistema, vault, records o integraciones." />
      </AnimatedBlock>
    </div>
  </SceneShell>
);

const FlowItem: React.FC<{color: string; label: string}> = ({color, label}) => (
  <>
    <div
      style={{
        width: 18,
        height: 18,
        borderRadius: 999,
        background: color,
        boxShadow: `0 0 18px ${color}`,
      }}
    />
    <div style={{fontSize: 42, fontWeight: 700}}>{label}</div>
  </>
);

const SceneThree = () => (
  <SceneShell>
    <AnimatedBlock delay={4} fromY={24}>
      <Eyebrow>2 · project mode</Eyebrow>
    </AnimatedBlock>
    <AnimatedBlock delay={8} fromY={34}>
      <Title size={74}>Workflow versionado</Title>
    </AnimatedBlock>
    <AnimatedBlock delay={18} fromY={36}>
      <div style={{display: 'flex', alignItems: 'center', gap: 18, marginTop: 36}}>
        <FlowItem color={palette.cyan} label="Explore" />
        <div style={{fontSize: 34, color: palette.dim}}>→</div>
        <FlowItem color={palette.magenta} label="Proposal" />
        <div style={{fontSize: 34, color: palette.dim}}>→</div>
        <FlowItem color={palette.orange} label="Apply" />
        <div style={{fontSize: 34, color: palette.dim}}>→</div>
        <FlowItem color={palette.lime} label="Verify" />
      </div>
    </AnimatedBlock>
    <AnimatedBlock delay={28} fromY={24}>
      <Subtitle style={{marginTop: 42, maxWidth: 1280}}>
        AgentHub baja el trabajo a proyectos, sesiones y cambios con trazabilidad.
        El resultado queda listo para test, release notes y commit.
      </Subtitle>
    </AnimatedBlock>
  </SceneShell>
);

const SceneFour = () => (
  <SceneShell>
    <AnimatedBlock delay={4} fromY={24}>
      <Eyebrow>3 · operación segura</Eyebrow>
    </AnimatedBlock>
    <AnimatedBlock delay={8} fromY={34}>
      <Title size={74}>Skills · Cron · Vault · Safe restart</Title>
    </AnimatedBlock>
    <div style={{display: 'flex', gap: 24, marginTop: 42}}>
      <AnimatedBlock delay={18} fromX={-60} fromY={0}>
        <Card step="automation" label="Cron & skills" />
      </AnimatedBlock>
      <AnimatedBlock delay={24} fromY={60}>
        <Card step="secrets" label="Vault" />
      </AnimatedBlock>
      <AnimatedBlock delay={30} fromX={60} fromY={0}>
        <Card step="deploy" label="Safe restart" />
      </AnimatedBlock>
    </div>
    <AnimatedBlock delay={34} fromY={18}>
      <FooterLine left="un cerebro" right="web + whatsapp + system automation" rightAccent />
    </AnimatedBlock>
  </SceneShell>
);

export const AgentHubWorkflows: React.FC = () => {
  const frame = useCurrentFrame();
  const bgGlowX = interpolate(frame, [0, 480], [260, 1180], {
    extrapolateRight: 'clamp',
  });
  const bgGlowY = interpolate(frame, [0, 480], [120, 80], {
    extrapolateRight: 'clamp',
  });
  const scanTop = interpolate(frame, [0, 480], [260, 360], {
    easing: Easing.inOut(Easing.ease),
    extrapolateRight: 'clamp',
  });

  return (
    <AbsoluteFill
      style={{
        background:
          'radial-gradient(circle at 20% 10%, rgba(94,240,255,0.16), transparent 32%), radial-gradient(circle at 80% 0%, rgba(255,78,214,0.12), transparent 30%), linear-gradient(180deg, #0b1029 0%, #060814 55%, #02030a 100%)',
        color: palette.fg,
      }}
    >
      <div style={gridStyle} />
      <div
        style={{
          position: 'absolute',
          width: 520,
          height: 520,
          borderRadius: 999,
          background: 'radial-gradient(circle, rgba(94,240,255,0.18), transparent 68%)',
          left: bgGlowX,
          top: bgGlowY,
          filter: 'blur(8px)',
        }}
      />
      <div
        style={{
          position: 'absolute',
          left: 0,
          right: 0,
          top: scanTop,
          height: 2,
          background: 'linear-gradient(90deg, transparent, rgba(94,240,255,0.85), transparent)',
          opacity: 0.22,
          filter: 'blur(1px)',
        }}
      />

      <Sequence from={0} durationInFrames={120}>
        <SceneOne />
      </Sequence>
      <Sequence from={120} durationInFrames={120}>
        <SceneTwo />
      </Sequence>
      <Sequence from={240} durationInFrames={120}>
        <SceneThree />
      </Sequence>
      <Sequence from={360} durationInFrames={120}>
        <SceneFour />
      </Sequence>
    </AbsoluteFill>
  );
};
