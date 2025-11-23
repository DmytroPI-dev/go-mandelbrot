import { useState, useRef, useEffect, useCallback } from 'react';
import axios from 'axios';

const API_URL = import.meta.env.VITE_API_URL;

// --- FIX 1: Decouple calculation resolution from display size ---
// We will always calculate a fractal of this size for predictable performance.
const CALCULATION_WIDTH = 800;
const CALCULATION_HEIGHT = 800;

export const FractalCanvas = () => {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const highResCanvasRef = useRef<HTMLCanvasElement | null>(null);
    const previewCanvasRef = useRef<HTMLCanvasElement | null>(null);

    const [isLoading, setIsLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const [viewport, setViewport] = useState({
        centerX: -0.75,
        centerY: 0.0,
        height: 2.5,
    });

    const [isPanning, setIsPanning] = useState(false);
    const [panStart, setPanStart] = useState({ x: 0, y: 0 });

    const fetchAndDrawFractal = useCallback(async (currentViewport: typeof viewport) => {
        setIsLoading(true);
        setError(null);

        const canvas = highResCanvasRef.current;
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;

        // Use our fixed calculation size for the API call
        const aspectRatio = CALCULATION_WIDTH / CALCULATION_HEIGHT;
        const viewportWidth = currentViewport.height * aspectRatio;
        const posX = currentViewport.centerX - viewportWidth / 2;
        const posY = currentViewport.centerY - currentViewport.height / 2;

        try {
            const requestUrl = `${API_URL}?width=${CALCULATION_WIDTH}&height_px=${CALCULATION_HEIGHT}&posX=${posX}&posY=${posY}&height=${currentViewport.height}&samples=4&maxIter=350`;
            const response = await axios.get(requestUrl, { responseType: 'arraybuffer' });

            const pixelData = new Uint8ClampedArray(response.data);
            const imageData = new ImageData(pixelData, CALCULATION_WIDTH, CALCULATION_HEIGHT);
            ctx.putImageData(imageData, 0, 0);

            const previewCanvas = previewCanvasRef.current;
            if (previewCanvas) {
                previewCanvas.getContext('2d')?.clearRect(0, 0, previewCanvas.width, previewCanvas.height);
            }

        } catch (err) {
            console.error("Failed to fetch fractal data:", err);
            setError("Failed to load fractal. Check the console for details.");
        } finally {
            setIsLoading(false);
        }
    }, []);

    useEffect(() => {
        const debounceTimeout = setTimeout(() => {
            fetchAndDrawFractal(viewport);
        }, 400);
        return () => clearTimeout(debounceTimeout);
    }, [viewport, fetchAndDrawFractal]);

    // Set the fixed pixel buffer size for our canvases on mount
    useEffect(() => {
        const canvases = [highResCanvasRef.current, previewCanvasRef.current];
        canvases.forEach(canvas => {
            if (canvas) {
                canvas.width = CALCULATION_WIDTH;
                canvas.height = CALCULATION_HEIGHT;
            }
        });
        fetchAndDrawFractal(viewport);
    }, [fetchAndDrawFractal, viewport]);

    // --- FIX 2: Manually add wheel event listener to set passive: false ---
    useEffect(() => {
        const container = containerRef.current;
        if (!container) return;

        // The handleWheel logic is now inside this useEffect
        const handleWheel = (e: WheelEvent) => {
            e.preventDefault();
            const zoomFactor = 1.2;
            const newHeight = e.deltaY < 0 ? viewport.height / zoomFactor : viewport.height * zoomFactor;

            const previewCtx = previewCanvasRef.current?.getContext('2d');
            const highResCanvas = highResCanvasRef.current;
            if (previewCtx && highResCanvas) {
                const scale = newHeight > viewport.height ? 1 / zoomFactor : zoomFactor;
                const { width, height } = previewCtx.canvas;
                previewCtx.clearRect(0, 0, width, height);
                previewCtx.save();
                previewCtx.translate(width / 2, height / 2);
                previewCtx.scale(scale, scale);
                previewCtx.translate(-width / 2, -height / 2);
                previewCtx.drawImage(highResCanvas, 0, 0);
                previewCtx.restore();
            }
            setViewport(prev => ({ ...prev, height: newHeight }));
        };

        container.addEventListener('wheel', handleWheel, { passive: false });

        return () => {
            container.removeEventListener('wheel', handleWheel);
        };
    }, [viewport]); // Re-attach listener if viewport changes to get latest value

    // --- Panning handlers remain mostly the same ---
    const handleMouseDown = (e: React.MouseEvent<HTMLDivElement>) => {
        setIsPanning(true);
        setPanStart({ x: e.clientX, y: e.clientY });
    };

    const handleMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
        if (!isPanning) return;
        const dx = e.clientX - panStart.x;
        const dy = e.clientY - panStart.y;

        const previewCtx = previewCanvasRef.current?.getContext('2d');
        const highResCanvas = highResCanvasRef.current;
        if (previewCtx && highResCanvas) {
            const { width, height } = previewCtx.canvas;
            previewCtx.clearRect(0, 0, width, height);
            // Scale the pan distance based on the display size vs calculation size
            const scaleX = width / (containerRef.current?.clientWidth || 1);
            const scaleY = height / (containerRef.current?.clientHeight || 1);
            previewCtx.drawImage(highResCanvas, dx * scaleX, dy * scaleY);
        }
    };

    const handleMouseUp = (e: React.MouseEvent<HTMLDivElement>) => {
        if (!isPanning) return;
        setIsPanning(false);

        const dx = e.clientX - panStart.x;
        const dy = e.clientY - panStart.y;

        const pixelsPerComplexUnit = (containerRef.current?.clientHeight || 0) / viewport.height;
        const deltaComplexX = dx / pixelsPerComplexUnit;
        const deltaComplexY = dy / pixelsPerComplexUnit;

        setViewport(prev => ({
            ...prev,
            centerX: prev.centerX - deltaComplexX,
            centerY: prev.centerY - deltaComplexY,
        }));
    };

    const zoom = (factor: number) => {
        setViewport(prev => ({ ...prev, height: prev.height * factor }));
    };
    const resetView = () => {
        setViewport({ centerX: -0.75, centerY: 0.0, height: 2.5 });
    };

    return (
        <div
            ref={containerRef}
            className="fractal-container"
            onMouseDown={handleMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
            onMouseLeave={handleMouseUp}
        >
            <div className="ui-overlay">
                <button onClick={() => zoom(0.8)}>Zoom In</button>
                <button onClick={() => zoom(1.2)}>Zoom Out</button>
                <button onClick={resetView}>Reset View</button>
                {isLoading && <p>Generating...</p>}
                {error && <p style={{ color: 'red' }}>{error}</p>}
                <p>Scale (Height): {viewport.height.toExponential(2)}</p>
            </div>

            <canvas ref={highResCanvasRef} style={{ zIndex: 1, cursor: isPanning ? 'grabbing' : 'grab' }} />
            <canvas ref={previewCanvasRef} style={{ zIndex: 2, pointerEvents: 'none' }} />
        </div>
    );
};