# Cloud-Native Mandelbrot Explorer

This project renders the Mandelbrot set using a modern, cloud-native architecture. It features a high-performance Go backend deployed as a serverless function on Google Cloud, and an interactive React frontend for exploring the fractal.

## Architecture

- **Backend**: A concurrent Go application that generates the fractal image. It's designed to be deployed as a Google Cloud Function for scalability and cost-efficiency.
- **Frontend**: A React application (built with Chakra UI / Material Design) that provides a user interface to pan, zoom, and customize the fractal rendering.
- **CI/CD**: Automated testing and deployment pipelines using GitHub Actions.

## Original Project

The core parallel processing logic for Go was adapted from the [parallel-mandelbrot-go](https://github.com/daniellferreira/parallel-mandelbrot-go) project by Daniela Ferreira. This new version refactors the original desktop application into a scalable web service.
