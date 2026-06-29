import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Badge,
  Box,
  Button,
  Field,
  Flex,
  Grid,
  Heading,
  NativeSelect,
  SimpleGrid,
  Stack,
  Text,
} from '@chakra-ui/react';
import axios from 'axios';

const API_URL = import.meta.env.VITE_API_URL;
const RENDER_SIZE = 800;
const DEFAULT_VIEWPORT = {
  centerX: -0.75,
  centerY: 0,
  height: 2.5,
};

type Viewport = typeof DEFAULT_VIEWPORT;

type QualityPreset = {
  id: string;
  label: string;
  samples: number;
  maxIter: number;
};

const QUALITY_PRESETS: QualityPreset[] = [
  { id: 'draft', label: 'Draft', samples: 1, maxIter: 160 },
  { id: 'balanced', label: 'Balanced', samples: 3, maxIter: 300 },
  { id: 'detail', label: 'Detail', samples: 5, maxIter: 520 },
];

function parseInitialViewport(): Viewport {
  const params = new URLSearchParams(window.location.search);
  const centerX = params.has('x') ? Number(params.get('x')) : Number.NaN;
  const centerY = params.has('y') ? Number(params.get('y')) : Number.NaN;
  const height = params.has('h') ? Number(params.get('h')) : Number.NaN;

  return {
    centerX: Number.isFinite(centerX) ? centerX : DEFAULT_VIEWPORT.centerX,
    centerY: Number.isFinite(centerY) ? centerY : DEFAULT_VIEWPORT.centerY,
    height: Number.isFinite(height) && height > 0 ? height : DEFAULT_VIEWPORT.height,
  };
}

function formatNumber(value: number) {
  if (Math.abs(value) >= 1000 || Math.abs(value) < 0.01) {
    return value.toExponential(3);
  }
  return value.toFixed(5);
}

export const FractalCanvas = () => {
  const stageRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const panStartRef = useRef<{ x: number; y: number } | null>(null);

  const [viewport, setViewport] = useState<Viewport>(parseInitialViewport);
  const [qualityId, setQualityId] = useState('balanced');
  const [isRendering, setIsRendering] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [renderTimeMs, setRenderTimeMs] = useState<number | null>(null);
  const [bytesRendered, setBytesRendered] = useState<number | null>(null);
  const [draftOffset, setDraftOffset] = useState({ x: 0, y: 0 });
  const [isPanning, setIsPanning] = useState(false);

  const quality = useMemo(
    () => QUALITY_PRESETS.find((preset) => preset.id === qualityId) ?? QUALITY_PRESETS[1],
    [qualityId],
  );

  const viewportWidth = viewport.height;
  const topLeftX = viewport.centerX - viewportWidth / 2;
  const topLeftY = viewport.centerY - viewport.height / 2;

  const renderFractal = useCallback(
    async (signal: AbortSignal) => {
      const canvas = canvasRef.current;
      if (!canvas) {
        return;
      }

      const ctx = canvas.getContext('2d');
      if (!ctx) {
        setError('Canvas rendering is not available in this browser.');
        return;
      }

      if (!API_URL) {
        setError('VITE_API_URL is not configured.');
        return;
      }

      setIsRendering(true);
      setError(null);
      setDraftOffset({ x: 0, y: 0 });

      const startedAt = performance.now();
      const params = new URLSearchParams({
        width: String(RENDER_SIZE),
        height_px: String(RENDER_SIZE),
        posX: String(topLeftX),
        posY: String(topLeftY),
        height: String(viewport.height),
        samples: String(quality.samples),
        maxIter: String(quality.maxIter),
        numBlocks: '64',
        numThreads: '16',
      });

      try {
        const response = await axios.get<ArrayBuffer>(`${API_URL}?${params.toString()}`, {
          responseType: 'arraybuffer',
          signal,
        });

        const pixelData = new Uint8ClampedArray(response.data);
        const imageData = new ImageData(pixelData, RENDER_SIZE, RENDER_SIZE);
        ctx.putImageData(imageData, 0, 0);
        setRenderTimeMs(Math.round(performance.now() - startedAt));
        setBytesRendered(pixelData.byteLength);
      } catch (err) {
        if (axios.isCancel(err) || signal.aborted) {
          return;
        }
        console.error('Failed to fetch fractal data:', err);
        setError('Render failed. Check the API endpoint and CORS configuration.');
      } finally {
        if (!signal.aborted) {
          setIsRendering(false);
        }
      }
    },
    [quality.maxIter, quality.samples, topLeftX, topLeftY, viewport.height],
  );

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    canvas.width = RENDER_SIZE;
    canvas.height = RENDER_SIZE;
  }, []);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    params.set('x', viewport.centerX.toPrecision(8));
    params.set('y', viewport.centerY.toPrecision(8));
    params.set('h', viewport.height.toPrecision(8));
    window.history.replaceState(null, '', `${window.location.pathname}?${params.toString()}`);
  }, [viewport]);

  useEffect(() => {
    const controller = new AbortController();
    const timeout = window.setTimeout(() => {
      void renderFractal(controller.signal);
    }, 180);

    return () => {
      window.clearTimeout(timeout);
      controller.abort();
    };
  }, [renderFractal]);

  const zoomAt = useCallback((factor: number, clientX?: number, clientY?: number) => {
    setViewport((current) => {
      const stage = stageRef.current;
      if (!stage || clientX === undefined || clientY === undefined) {
        return { ...current, height: current.height * factor };
      }

      const rect = stage.getBoundingClientRect();
      const normalizedX = (clientX - rect.left) / rect.width - 0.5;
      const normalizedY = (clientY - rect.top) / rect.height - 0.5;
      const beforeX = current.centerX + normalizedX * current.height;
      const beforeY = current.centerY + normalizedY * current.height;
      const nextHeight = current.height * factor;

      return {
        centerX: beforeX - normalizedX * nextHeight,
        centerY: beforeY - normalizedY * nextHeight,
        height: nextHeight,
      };
    });
  }, []);

  const handleWheel = useCallback(
    (event: React.WheelEvent<HTMLDivElement>) => {
      event.preventDefault();
      zoomAt(event.deltaY < 0 ? 0.82 : 1.22, event.clientX, event.clientY);
    },
    [zoomAt],
  );

  const handlePointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    event.currentTarget.setPointerCapture(event.pointerId);
    panStartRef.current = { x: event.clientX, y: event.clientY };
    setIsPanning(true);
  };

  const handlePointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const panStart = panStartRef.current;
    if (!panStart) {
      return;
    }

    setDraftOffset({
      x: event.clientX - panStart.x,
      y: event.clientY - panStart.y,
    });
  };

  const finishPan = (event: React.PointerEvent<HTMLDivElement>) => {
    const panStart = panStartRef.current;
    if (!panStart) {
      return;
    }

    const stage = stageRef.current;
    const rect = stage?.getBoundingClientRect();
    const dx = event.clientX - panStart.x;
    const dy = event.clientY - panStart.y;
    const unitsPerPixel = rect ? viewport.height / rect.height : 0;

    panStartRef.current = null;
    setIsPanning(false);
    setDraftOffset({ x: 0, y: 0 });
    setViewport((current) => ({
      ...current,
      centerX: current.centerX - dx * unitsPerPixel,
      centerY: current.centerY - dy * unitsPerPixel,
    }));
  };

  const resetView = () => {
    setViewport(DEFAULT_VIEWPORT);
  };

  const copyShareLink = async () => {
    await navigator.clipboard.writeText(window.location.href);
  };

  const byteLabel = bytesRendered === null ? 'n/a' : `${(bytesRendered / 1024).toFixed(0)} KB`;
  const timeLabel = renderTimeMs === null ? 'n/a' : `${renderTimeMs} ms`;
  const metrics = [
    ['Center X', formatNumber(viewport.centerX)],
    ['Center Y', formatNumber(viewport.centerY)],
    ['Height', viewport.height.toExponential(3)],
    ['Iterations', String(quality.maxIter)],
    ['Samples', String(quality.samples)],
    ['Payload', byteLabel],
    ['Render', timeLabel],
  ];

  return (
    <Box
      as="main"
      minH="100vh"
      p={{ base: '10px', md: '20px' }}
      bg="#111315"
      color="#f4f0e8"
      backgroundImage="linear-gradient(135deg, rgba(77, 163, 126, 0.12), transparent 34%), linear-gradient(315deg, rgba(207, 124, 68, 0.12), transparent 36%)"
    >
      <Stack
        as="section"
        aria-label="Mandelbrot explorer"
        gap="4"
        w="min(1280px, 100%)"
        h={{ base: 'auto', lg: 'calc(100vh - 40px)' }}
        minH={{ base: 'calc(100vh - 20px)', md: 'calc(100vh - 40px)' }}
        mx="auto"
      >
        <Flex align={{ base: 'flex-start', md: 'flex-end' }} justify="space-between" gap="4" direction={{ base: 'column', md: 'row' }}>
          <Box>
            <Text mb="1" color="#8fbda9" fontSize="xs" fontWeight="bold" textTransform="uppercase">
              AWS Serverless Renderer
            </Text>
            <Heading as="h1" color="#fff9ee" fontSize={{ base: '3xl', md: '5xl' }} lineHeight="1">
              Mandelbrot Explorer
            </Heading>
          </Box>
          <Badge
            display="inline-flex"
            alignItems="center"
            gap="2"
            minW="112px"
            justifyContent="center"
            px="3"
            py="2"
            rounded="md"
            borderWidth="1px"
            borderColor="whiteAlpha.200"
            bg="rgba(23, 26, 28, 0.9)"
            color="#d9d0c1"
            textTransform="none"
            fontSize="sm"
          >
            <Box boxSize="9px" rounded="full" bg={isRendering ? '#cf7c44' : '#4da37e'} boxShadow={`0 0 16px ${isRendering ? 'rgba(207, 124, 68, 0.9)' : 'rgba(77, 163, 126, 0.8)'}`} />
            {isRendering ? 'Rendering' : 'Ready'}
          </Badge>
        </Flex>

        <Grid templateColumns={{ base: '1fr', lg: '280px minmax(0, 1fr)' }} gap="4" minH="0" flex="1">
          <Stack
            as="aside"
            aria-label="Render controls"
            alignSelf="start"
            gap="4"
            p="4"
            order={{ base: 2, lg: 0 }}
            borderWidth="1px"
            borderColor="whiteAlpha.200"
            rounded="md"
            bg="rgba(20, 22, 24, 0.88)"
            boxShadow="0 18px 50px rgba(0, 0, 0, 0.22)"
          >
            <Field.Root>
              <Field.Label color="#d9d0c1" fontSize="xs" fontWeight="bold" textTransform="uppercase">
                Quality
              </Field.Label>
              <NativeSelect.Root>
                <NativeSelect.Field value={qualityId} onChange={(event) => setQualityId(event.target.value)} bg="#202326" borderColor="whiteAlpha.300" color="#fff9ee">
                  {QUALITY_PRESETS.map((preset) => (
                    <option key={preset.id} value={preset.id}>
                      {preset.label}
                    </option>
                  ))}
                </NativeSelect.Field>
                <NativeSelect.Indicator />
              </NativeSelect.Root>
            </Field.Root>

            <SimpleGrid columns={2} gap="2" aria-label="Viewport actions">
              <Button type="button" onClick={() => zoomAt(0.78)} title="Zoom in" aria-label="Zoom in" bg="#202326" borderColor="whiteAlpha.300">
                +
              </Button>
              <Button type="button" onClick={() => zoomAt(1.28)} title="Zoom out" aria-label="Zoom out" bg="#202326" borderColor="whiteAlpha.300">
                -
              </Button>
              <Button type="button" onClick={resetView} title="Reset view" aria-label="Reset view" bg="#202326" borderColor="whiteAlpha.300">
                reset
              </Button>
              <Button type="button" onClick={() => void copyShareLink()} title="Copy share link" aria-label="Copy share link" bg="#202326" borderColor="whiteAlpha.300">
                link
              </Button>
            </SimpleGrid>

            <Stack gap="0" as="dl">
              {metrics.map(([label, value], index) => (
                <Flex
                  key={label}
                  as="div"
                  justify="space-between"
                  gap="3"
                  py="2.5"
                  borderBottomWidth={index === metrics.length - 1 ? '0' : '1px'}
                  borderColor="whiteAlpha.200"
                >
                  <Text as="dt" color="#a69d91" fontSize="xs">
                    {label}
                  </Text>
                  <Text as="dd" m="0" color="#fff9ee" fontFamily="mono" fontSize="sm" textAlign="right" overflowWrap="anywhere">
                    {value}
                  </Text>
                </Flex>
              ))}
            </Stack>
          </Stack>

          <Box
            ref={stageRef}
            className="fractal-stage"
            data-panning={isPanning ? 'true' : undefined}
            onWheel={handleWheel}
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={finishPan}
            onPointerCancel={finishPan}
            position="relative"
            display="grid"
            placeItems="center"
            minH={{ base: '320px', lg: '0' }}
            overflow="hidden"
            borderWidth="1px"
            borderColor="whiteAlpha.200"
            rounded="md"
            bg="#080909"
            cursor={isPanning ? 'grabbing' : 'grab'}
            touchAction="none"
          >
            <canvas
              ref={canvasRef}
              style={{ transform: `translate(${draftOffset.x}px, ${draftOffset.y}px)` }}
              aria-label="Rendered Mandelbrot set"
            />
            {isRendering && <Box className="render-scrim" aria-hidden="true" />}
            {error && (
              <Box position="absolute" right="4" bottom="4" maxW="min(420px, calc(100% - 32px))" px="3" py="2.5" borderWidth="1px" borderColor="rgba(220, 91, 75, 0.5)" rounded="md" bg="rgba(54, 21, 18, 0.92)" color="#ffe2dc" fontSize="sm">
                {error}
              </Box>
            )}
          </Box>
        </Grid>
      </Stack>
    </Box>
  );
};
