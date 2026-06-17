class TuoshanDam3D {
    constructor(containerId) {
        this.container = document.getElementById(containerId);
        this.simulationData = null;
        this.sensorConfigs = [];
        this.sensorDataMap = {};
        this.options = {
            showStreamlines: true,
            showPressureCloud: true,
            showWireframe: false,
            showSensors: true,
            particleCount: 200,
            particleSpeed: 5
        };

        this.damLength = 113.7;
        this.damHeight = 3.85;
        this.damTopWidth = 4.8;
        this.upstreamSlope = 0.35;
        this.downstreamSlope = 0.6;
        this.foundationDepth = 5.0;

        this.streamParticles = [];
        this.particleGeometry = null;
        this.particleMaterial = null;
        this.particleSystem = null;
        this.gridMesh = null;
        this.damGroup = null;
        this.waterUpstream = null;
        this.waterDownstream = null;
        this.sensorMarkers = [];
        this.blanketMesh = null;

        this.upstreamWL = 6.8;
        this.downstreamWL = 2.9;

        this.init();
    }

    init() {
        this.width = this.container.clientWidth;
        this.height = this.container.clientHeight;

        this.scene = new THREE.Scene();
        this.scene.background = new THREE.Color(0x0a1628);
        this.scene.fog = new THREE.Fog(0x0a1628, 150, 300);

        this.camera = new THREE.PerspectiveCamera(50, this.width / this.height, 0.1, 1000);
        this.camera.position.set(90, 35, 80);
        this.camera.lookAt(55, 0, 0);

        this.renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
        this.renderer.setSize(this.width, this.height);
        this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
        this.renderer.shadowMap.enabled = true;
        this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
        this.container.appendChild(this.renderer.domElement);

        this.setupLights();
        this.buildDamGeometry();
        this.buildFoundation();
        this.buildWater();
        this.buildGridHelper();
        this.buildStreamParticles();

        this.setupControls();

        window.addEventListener('resize', () => this.onWindowResize());

        this.animate();
    }

    setupLights() {
        const ambientLight = new THREE.AmbientLight(0x404060, 0.6);
        this.scene.add(ambientLight);

        const dirLight = new THREE.DirectionalLight(0xffffff, 1.0);
        dirLight.position.set(60, 80, 40);
        dirLight.castShadow = true;
        dirLight.shadow.mapSize.width = 2048;
        dirLight.shadow.mapSize.height = 2048;
        dirLight.shadow.camera.near = 0.5;
        dirLight.shadow.camera.far = 300;
        dirLight.shadow.camera.left = -80;
        dirLight.shadow.camera.right = 80;
        dirLight.shadow.camera.top = 80;
        dirLight.shadow.camera.bottom = -80;
        this.scene.add(dirLight);

        const fillLight = new THREE.DirectionalLight(0x6688cc, 0.4);
        fillLight.position.set(-50, 30, -20);
        this.scene.add(fillLight);

        const pointLight = new THREE.PointLight(0x44aaff, 0.5, 100);
        pointLight.position.set(56, -2, 0);
        this.scene.add(pointLight);
    }

    damProfile(y) {
        const topCenterX = this.damLength / 2.0;
        const xTopStart = topCenterX - this.damTopWidth / 2.0;
        const xTopEnd = topCenterX + this.damTopWidth / 2.0;

        const relativeY = this.damHeight - y;
        if (relativeY <= 0) {
            return { xStart: xTopStart, xEnd: xTopEnd };
        }

        let xStart = xTopStart - relativeY * this.upstreamSlope;
        let xEnd = xTopEnd + relativeY * this.downstreamSlope;

        if (xStart < 0) xStart = 0;
        if (xEnd > this.damLength) xEnd = this.damLength;

        return { xStart, xEnd };
    }

    buildDamGeometry() {
        this.damGroup = new THREE.Group();

        const depth = 15;
        const segmentsY = 40;
        const segmentsZ = 4;

        const damMaterial = new THREE.MeshStandardMaterial({
            color: 0x8b7355,
            roughness: 0.85,
            metalness: 0.05,
            transparent: true,
            opacity: 0.92
        });

        const coreMaterial = new THREE.MeshStandardMaterial({
            color: 0x6b4423,
            roughness: 0.9,
            metalness: 0.0
        });

        const damGeo = new THREE.BufferGeometry();
        const vertices = [];
        const normals = [];
        const indices = [];

        const yMin = -this.foundationDepth;
        const yMax = this.damHeight;
        const dy = (yMax - yMin) / segmentsY;

        for (let iy = 0; iy <= segmentsY; iy++) {
            const y = yMin + iy * dy;
            const { xStart, xEnd } = this.damProfile(y);

            const actualXStart = y <= 0 ? 0 : xStart;
            const actualXEnd = y <= 0 ? this.damLength : xEnd;

            for (let iz = 0; iz <= segmentsZ; iz++) {
                const z = -depth / 2 + (depth / segmentsZ) * iz;

                vertices.push(actualXStart, y, z);
                vertices.push(actualXEnd, y, z);

                normals.push(0, 0, 0);
                normals.push(0, 0, 0);
            }
        }

        const cols = 2 * (segmentsZ + 1);
        for (let iy = 0; iy < segmentsY; iy++) {
            for (let ic = 0; ic < cols - 2; ic += 2) {
                const a = iy * cols + ic;
                const b = a + 1;
                const c = (iy + 1) * cols + ic;
                const d = c + 1;

                indices.push(a, c, b);
                indices.push(b, c, d);
            }
        }

        for (let iy = 0; iy <= segmentsY; iy++) {
            for (let iz = 0; iz < segmentsZ; iz++) {
                const baseIdx = iy * cols + iz * 2;
                for (let offset = 0; offset <= 1; offset++) {
                    const a = baseIdx + offset;
                    const b = baseIdx + offset + 2;
                    const c = a + cols;
                    const d = b + cols;

                    if (iz === 0 && offset === 0) {
                        indices.push(a, b, c);
                        indices.push(b, d, c);
                    } else if (iz === segmentsZ - 1 && offset === 1) {
                    }
                }
            }
        }

        damGeo.setAttribute('position', new THREE.Float32BufferAttribute(vertices, 3));
        damGeo.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3));
        damGeo.setIndex(indices);
        damGeo.computeVertexNormals();

        const damMesh = new THREE.Mesh(damGeo, damMaterial);
        damMesh.castShadow = true;
        damMesh.receiveShadow = true;
        this.damGroup.add(damMesh);

        if (this.options.showWireframe) {
            const wireGeo = new THREE.WireframeGeometry(damGeo);
            const wireMat = new THREE.LineBasicMaterial({ color: 0x334466, opacity: 0.3, transparent: true });
            const wireMesh = new THREE.LineSegments(wireGeo, wireMat);
            this.damGroup.add(wireMesh);
        }

        this.buildCoreWall(depth);
        this.buildStoneLayers(depth);

        this.scene.add(this.damGroup);
    }

    buildCoreWall(depth) {
        const topCenterX = this.damLength / 2.0;
        const coreWidth = 1.5;

        const coreShape = new THREE.Shape();
        coreShape.moveTo(topCenterX - coreWidth / 2, 0);
        coreShape.lineTo(topCenterX - coreWidth / 2 - 0.3, this.damHeight);
        coreShape.lineTo(topCenterX + coreWidth / 2 + 0.3, this.damHeight);
        coreShape.lineTo(topCenterX + coreWidth / 2, 0);
        coreShape.lineTo(topCenterX - coreWidth / 2, 0);

        const extrudeSettings = { depth: depth, bevelEnabled: false };
        const coreGeo = new THREE.ExtrudeGeometry(coreShape, extrudeSettings);
        coreGeo.translate(0, 0, -depth / 2);

        const coreMat = new THREE.MeshStandardMaterial({
            color: 0x5c3a1e,
            roughness: 0.95,
            transparent: true,
            opacity: 0.85
        });

        const coreMesh = new THREE.Mesh(coreGeo, coreMat);
        coreMesh.castShadow = true;
        this.damGroup.add(coreMesh);
    }

    buildStoneLayers(depth) {
        const stoneMat = new THREE.MeshStandardMaterial({
            color: 0x666666,
            roughness: 0.9,
            flatShading: true
        });

        for (let i = 0; i < 30; i++) {
            const y = (this.damHeight / 30) * i;
            const { xStart, xEnd } = this.damProfile(y + 0.1);
            const { xStart: xStartNext, xEnd: xEndNext } = this.damProfile(y + this.damHeight / 30);

            if (i % 5 === 0) {
                const stoneGeo = new THREE.BoxGeometry(0.6, 0.25, 0.5);
                const stone = new THREE.Mesh(stoneGeo, stoneMat);
                stone.position.set(
                    xStart + 0.3 + (i % 3) * 0.1,
                    y + 0.12,
                    (i % 5 - 2) * 1.5
                );
                stone.rotation.y = (i % 7) * 0.1;
                this.damGroup.add(stone);

                const stone2 = new THREE.Mesh(stoneGeo, stoneMat);
                stone2.position.set(
                    xEnd - 0.3 - (i % 4) * 0.1,
                    y + 0.12,
                    (i % 3 - 1) * 2.0
                );
                stone2.rotation.y = -(i % 5) * 0.15;
                this.damGroup.add(stone2);
            }
        }
    }

    buildFoundation() {
        const depth = 15;
        const foundationGeo = new THREE.BoxGeometry(this.damLength + 20, this.foundationDepth, depth + 10);
        const foundationMat = new THREE.MeshStandardMaterial({
            color: 0x4a4a3e,
            roughness: 0.95
        });
        const foundation = new THREE.Mesh(foundationGeo, foundationMat);
        foundation.position.set(this.damLength / 2, -this.foundationDepth / 2, 0);
        foundation.receiveShadow = true;
        this.scene.add(foundation);

        const riverBedGeo = new THREE.PlaneGeometry(this.damLength + 60, 40);
        const riverBedMat = new THREE.MeshStandardMaterial({
            color: 0x556b52,
            roughness: 0.9
        });
        const riverBed = new THREE.Mesh(riverBedGeo, riverBedMat);
        riverBed.rotation.x = -Math.PI / 2;
        riverBed.position.set(this.damLength / 2, -0.01, 0);
        riverBed.receiveShadow = true;
        this.scene.add(riverBed);
    }

    buildWater() {
        const depth = 15;

        const upstreamShape = new THREE.Shape();
        upstreamShape.moveTo(-20, 0);
        upstreamShape.lineTo(-20, this.upstreamWL);
        upstreamShape.lineTo(0, this.upstreamWL);
        upstreamShape.lineTo(0, 0);
        upstreamShape.lineTo(-20, 0);

        const waterExtrude = { depth: depth, bevelEnabled: false };
        const upWaterGeo = new THREE.ExtrudeGeometry(upstreamShape, waterExtrude);
        upWaterGeo.translate(0, 0, -depth / 2);

        const waterMat = new THREE.MeshStandardMaterial({
            color: 0x2980b9,
            transparent: true,
            opacity: 0.65,
            roughness: 0.1,
            metalness: 0.1
        });

        this.waterUpstream = new THREE.Mesh(upWaterGeo, waterMat);
        this.scene.add(this.waterUpstream);

        const downstreamShape = new THREE.Shape();
        downstreamShape.moveTo(this.damLength, 0);
        downstreamShape.lineTo(this.damLength, this.downstreamWL);
        downstreamShape.lineTo(this.damLength + 30, this.downstreamWL);
        downstreamShape.lineTo(this.damLength + 30, 0);
        downstreamShape.lineTo(this.damLength, 0);

        const downWaterGeo = new THREE.ExtrudeGeometry(downstreamShape, waterExtrude);
        downWaterGeo.translate(0, 0, -depth / 2);

        const downWaterMat = waterMat.clone();
        downWaterMat.opacity = 0.55;
        this.waterDownstream = new THREE.Mesh(downWaterGeo, downWaterMat);
        this.scene.add(this.waterDownstream);

        this.buildWaterSurface(-10, this.upstreamWL, 20, depth, waterMat);
        this.buildWaterSurface(this.damLength + 15, this.downstreamWL, 25, depth, waterMat);
    }

    buildWaterSurface(centerX, y, width, depth, material) {
        const geo = new THREE.PlaneGeometry(width, depth, 20, 8);
        const pos = geo.attributes.position;
        for (let i = 0; i < pos.count; i++) {
            const x = pos.getX(i);
            const z = pos.getZ(i);
            pos.setY(i, Math.sin(x * 0.5 + z * 0.3) * 0.05);
        }
        geo.computeVertexNormals();

        const waterMat = material.clone();
        waterMat.side = THREE.DoubleSide;
        const surface = new THREE.Mesh(geo, waterMat);
        surface.rotation.x = -Math.PI / 2;
        surface.position.set(centerX, y + 0.001, 0);
        surface.userData.isWaterSurface = true;
        surface.userData.baseY = y;
        this.scene.add(surface);
    }

    updateWaterLevels(upWL, downWL) {
        this.upstreamWL = upWL;
        this.downstreamWL = downWL;
        this.scene.remove(this.waterUpstream);
        this.scene.remove(this.waterDownstream);

        const toRemove = [];
        this.scene.traverse((obj) => {
            if (obj.userData && obj.userData.isWaterSurface) {
                toRemove.push(obj);
            }
        });
        toRemove.forEach(obj => this.scene.remove(obj));

        this.buildWater();
    }

    buildGridHelper() {
        const gridSize = this.damLength + 40;
        const gridDivisions = 50;
        const gridHelper = new THREE.GridHelper(gridSize, gridDivisions, 0x1a3a5a, 0x0e1e30);
        gridHelper.position.set(gridSize / 2 - 20, -this.foundationDepth - 0.001, 0);
        gridHelper.material.opacity = 0.4;
        gridHelper.material.transparent = true;
        this.scene.add(gridHelper);

        const axesHelper = new THREE.AxesHelper(10);
        axesHelper.position.set(-15, -this.foundationDepth, -12);
        this.scene.add(axesHelper);
    }

    buildStreamParticles() {
        this.particleGeometry = new THREE.BufferGeometry();
        const positions = new Float32Array(this.options.particleCount * 3);
        const colors = new Float32Array(this.options.particleCount * 3);
        const sizes = new Float32Array(this.options.particleCount);

        this.streamParticles = [];

        for (let i = 0; i < this.options.particleCount; i++) {
            const particle = this.createRandomParticle();
            this.streamParticles.push(particle);

            positions[i * 3] = particle.x;
            positions[i * 3 + 1] = particle.y;
            positions[i * 3 + 2] = particle.z;

            const color = new THREE.Color();
            color.setHSL(0.5 + particle.speed * 0.2, 0.9, 0.6);
            colors[i * 3] = color.r;
            colors[i * 3 + 1] = color.g;
            colors[i * 3 + 2] = color.b;

            sizes[i] = 0.15 + particle.speed * 0.1;
        }

        this.particleGeometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        this.particleGeometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));
        this.particleGeometry.setAttribute('size', new THREE.BufferAttribute(sizes, 1));

        this.particleMaterial = new THREE.PointsMaterial({
            size: 0.25,
            vertexColors: true,
            transparent: true,
            opacity: 0.8,
            blending: THREE.AdditiveBlending,
            sizeAttenuation: true
        });

        this.particleSystem = new THREE.Points(this.particleGeometry, this.particleMaterial);
        this.scene.add(this.particleSystem);
    }

    createRandomParticle() {
        const gradient = (this.upstreamWL - this.downstreamWL) / this.damLength;
        const baseSpeed = 0.002 + gradient * 0.01;

        let x, y, z;
        let attempts = 0;
        do {
            x = Math.random() * this.damLength;
            y = -this.foundationDepth + Math.random() * (this.upstreamWL + this.foundationDepth);
            z = (Math.random() - 0.5) * 10;
            attempts++;
        } while (!this.isInsideDamBody(x, y) && attempts < 50);

        if (attempts >= 50) {
            const { xStart } = this.damProfile(this.upstreamWL * 0.3);
            x = Math.max(0, xStart - 1) + Math.random() * 3;
            y = Math.random() * this.upstreamWL * 0.5;
            z = (Math.random() - 0.5) * 10;
        }

        const yFactor = Math.max(0, 1 - y / (this.upstreamWL + 2));

        return {
            x, y, z,
            vx: -baseSpeed * (0.5 + Math.random() * 0.5),
            vy: -baseSpeed * 0.2 * (0.5 + Math.random() * 0.5) * yFactor,
            vz: (Math.random() - 0.5) * baseSpeed * 0.3,
            speed: baseSpeed * 100,
            life: 1.0,
            maxLife: 0.8 + Math.random() * 0.4
        };
    }

    isInsideDamBody(x, y) {
        if (x < 0 || x > this.damLength) return false;
        if (y > this.upstreamWL + 0.5 || y < -this.foundationDepth - 0.5) return false;

        if (y <= 0) {
            return x >= 0 && x <= this.damLength;
        }

        const { xStart, xEnd } = this.damProfile(y);
        return x >= xStart && x <= xEnd;
    }

    constrainParticleToDamBody(p) {
        if (p.y <= 0) {
            p.x = Math.max(0, Math.min(this.damLength, p.x));
            return;
        }

        const { xStart, xEnd } = this.damProfile(p.y);

        if (p.x < xStart) {
            p.x = xStart + 0.01;
            p.vx = Math.abs(p.vx) * 0.3;
        }
        if (p.x > xEnd) {
            p.x = xEnd - 0.01;
            p.vx = -Math.abs(p.vx) * 0.3;
        }

        if (p.y > this.damHeight) {
            p.y = this.damHeight - 0.01;
            p.vy = -Math.abs(p.vy) * 0.3;
        }
        if (p.y < 0) {
            p.y = 0.01;
            p.vy = Math.abs(p.vy) * 0.1;
        }
    }

    updateParticles() {
        if (!this.options.showStreamlines || !this.particleGeometry) return;

        const positions = this.particleGeometry.attributes.position.array;
        const colors = this.particleGeometry.attributes.color.array;
        const speedMult = this.options.particleSpeed / 5;
        const maxSubSteps = 3;

        for (let i = 0; i < this.streamParticles.length; i++) {
            const p = this.streamParticles[i];

            if (this.simulationData && this.simulationData.grids) {
                this.adjustVelocityFromSimulation(p);
            }

            const effectiveSpeed = speedMult;
            const dt = 1.0 / maxSubSteps;

            for (let sub = 0; sub < maxSubSteps; sub++) {
                const oldX = p.x;
                const oldY = p.y;

                p.x += p.vx * effectiveSpeed * dt;
                p.y += p.vy * effectiveSpeed * dt;
                p.z += p.vz * effectiveSpeed * dt;

                if (!this.isInsideDamBody(p.x, p.y)) {
                    p.x = oldX;
                    p.y = oldY;

                    if (p.y > 0) {
                        const { xStart, xEnd } = this.damProfile(p.y);

                        const dLeft = p.x - xStart;
                        const dRight = xEnd - p.x;
                        const dTop = this.damHeight - p.y;
                        const dBottom = p.y;

                        const minDist = Math.min(
                            dLeft > 0 ? dLeft : Infinity,
                            dRight > 0 ? dRight : Infinity,
                            dTop > 0 ? dTop : Infinity,
                            dBottom > 0 ? dBottom : Infinity
                        );

                        if (minDist === dLeft || minDist === dRight) {
                            p.vx = -p.vx * 0.2;
                        } else {
                            p.vy = -p.vy * 0.2;
                        }

                        p.x += p.vx * effectiveSpeed * dt * 0.5;
                        p.y += p.vy * effectiveSpeed * dt * 0.5;

                        if (!this.isInsideDamBody(p.x, p.y)) {
                            p.x = oldX;
                            p.y = oldY;
                        }
                    } else {
                        p.vx = -p.vx * 0.2;
                        p.vy = -p.vy * 0.2;
                    }
                }

                this.constrainParticleToDamBody(p);
            }

            p.life -= 0.005 * speedMult;

            const outOfBounds = p.x < -5 || p.x > this.damLength + 5 ||
                p.y < -this.foundationDepth - 1 || p.y > this.upstreamWL + 1;

            if (outOfBounds || p.life <= 0) {
                Object.assign(p, this.createRandomParticle());
                if (this.simulationData && this.simulationData.grids) {
                    this.seedParticleOnUpstream(p);
                }
            }

            positions[i * 3] = p.x;
            positions[i * 3 + 1] = p.y;
            positions[i * 3 + 2] = p.z;

            const alpha = Math.min(1, p.life * 1.5);
            const speed = Math.sqrt(p.vx * p.vx + p.vy * p.vy + p.vz * p.vz) * 1000;
            const hue = 0.6 - Math.min(0.3, speed * 0.05);
            const color = new THREE.Color();
            color.setHSL(hue, 0.85, 0.5 + speed * 0.02);
            colors[i * 3] = color.r * alpha;
            colors[i * 3 + 1] = color.g * alpha;
            colors[i * 3 + 2] = color.b * alpha;
        }

        this.particleGeometry.attributes.position.needsUpdate = true;
        this.particleGeometry.attributes.color.needsUpdate = true;
    }

    adjustVelocityFromSimulation(p) {
        if (!this.simulationData.grids || this.simulationData.grids.length === 0) return;

        const closestGrid = this.findClosestGrid(p.x, p.y);
        if (closestGrid && closestGrid.velocity_magnitude > 0) {
            const scale = 0.5;
            p.vx = closestGrid.velocity_x * scale * 1000;
            p.vy = closestGrid.velocity_y * scale * 1000;
            p.speed = closestGrid.velocity_magnitude * 1000;
        }
    }

    findClosestGrid(x, y) {
        if (!this.simulationData.grids) return null;
        let closest = null;
        let minDist = Infinity;

        for (const g of this.simulationData.grids) {
            const dx = g.grid_x - x;
            const dy = g.grid_y - y;
            const dist = dx * dx + dy * dy;
            if (dist < minDist) {
                minDist = dist;
                closest = g;
            }
        }
        return closest;
    }

    seedParticleOnUpstream(p) {
        const { xStart } = this.damProfile(this.upstreamWL * 0.5);
        const safeXStart = Math.max(0, xStart - 1);

        let attempts = 0;
        do {
            p.x = safeXStart + Math.random() * 5;
            p.y = Math.random() * this.upstreamWL;
            attempts++;
        } while (!this.isInsideDamBody(p.x, p.y) && attempts < 20);

        if (attempts >= 20) {
            p.x = this.damLength * 0.1;
            p.y = -this.foundationDepth * 0.5;
        }

        p.vx = Math.abs(p.vx);
        p.vx = 0.003 + Math.random() * 0.005;
        p.vy = -0.001 + Math.random() * 0.002;
    }

    updatePressureCloud(simulationData) {
        if (this.gridMesh) {
            this.scene.remove(this.gridMesh);
            this.gridMesh.geometry.dispose();
            this.gridMesh.material.dispose();
        }

        if (!this.options.showPressureCloud || !simulationData || !simulationData.grids) {
            return;
        }

        const grids = simulationData.grids;
        if (grids.length === 0) return;

        const maxPP = simulationData.simulation ?
            (simulationData.simulation.max_pore_pressure || 80) : 80;

        const gridPoints = [];
        for (const g of grids) {
            if (!g.is_saturated) continue;
            gridPoints.push(g);
        }

        if (gridPoints.length === 0) return;

        const pressureGeo = new THREE.BufferGeometry();
        const positions = [];
        const colors = [];
        const indices = [];

        const uniqueX = [...new Set(gridPoints.map(g => g.grid_x))].sort((a, b) => a - b);
        const uniqueY = [...new Set(gridPoints.map(g => g.grid_y))].sort((a, b) => a - b);

        const gridMap = {};
        for (const g of gridPoints) {
            gridMap[`${g.grid_x.toFixed(2)}_${g.grid_y.toFixed(2)}`] = g;
        }

        const depth = 14;
        const zLayers = [-depth / 2, 0, depth / 2];

        for (const z of zLayers) {
            const layerOffset = positions.length / 3;

            for (const g of gridPoints) {
                const pressure = g.pore_pressure || 0;
                const normPressure = Math.min(1, pressure / (maxPP * 1.2));

                positions.push(g.grid_x, g.grid_y, z);

                const color = this.pressureToColor(normPressure);
                colors.push(color.r, color.g, color.b);
            }

            for (let i = 0; i < uniqueY.length - 1; i++) {
                for (let j = 0; j < uniqueX.length - 1; j++) {
                    const key00 = `${uniqueX[j].toFixed(2)}_${uniqueY[i].toFixed(2)}`;
                    const key10 = `${uniqueX[j + 1].toFixed(2)}_${uniqueY[i].toFixed(2)}`;
                    const key01 = `${uniqueX[j].toFixed(2)}_${uniqueY[i + 1].toFixed(2)}`;
                    const key11 = `${uniqueX[j + 1].toFixed(2)}_${uniqueY[i + 1].toFixed(2)}`;

                    if (gridMap[key00] && gridMap[key10] && gridMap[key01] && gridMap[key11]) {
                        const g00 = gridMap[key00];
                        const g10 = gridMap[key10];
                        const g01 = gridMap[key01];
                        const g11 = gridMap[key11];

                        const idx00 = gridPoints.indexOf(g00) + layerOffset;
                        const idx10 = gridPoints.indexOf(g10) + layerOffset;
                        const idx01 = gridPoints.indexOf(g01) + layerOffset;
                        const idx11 = gridPoints.indexOf(g11) + layerOffset;

                        indices.push(idx00, idx10, idx11);
                        indices.push(idx00, idx11, idx01);
                    }
                }
            }
        }

        pressureGeo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
        pressureGeo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
        pressureGeo.setIndex(indices);
        pressureGeo.computeVertexNormals();

        const pressureMat = new THREE.MeshStandardMaterial({
            vertexColors: true,
            transparent: true,
            opacity: 0.75,
            side: THREE.DoubleSide,
            roughness: 0.8
        });

        this.gridMesh = new THREE.Mesh(pressureGeo, pressureMat);
        this.scene.add(this.gridMesh);

        document.getElementById('legendMin').textContent = '0';
        document.getElementById('legendMid').textContent = (maxPP / 2).toFixed(0);
        document.getElementById('legendMax').textContent = maxPP.toFixed(0);
    }

    pressureToColor(norm) {
        const clamped = Math.max(0, Math.min(1, norm));
        let r, g, b;

        if (clamped < 0.15) {
            const t = clamped / 0.15;
            r = 0; g = t * 0.5; b = 0.5 + t * 0.5;
        } else if (clamped < 0.3) {
            const t = (clamped - 0.15) / 0.15;
            r = 0; g = 0.5 + t * 0.3; b = 1 - t * 0.5;
        } else if (clamped < 0.45) {
            const t = (clamped - 0.3) / 0.15;
            r = 0; g = 0.8 + t * 0.2; b = 0.5 - t * 0.5;
        } else if (clamped < 0.6) {
            const t = (clamped - 0.45) / 0.15;
            r = 0; g = 1; b = t * 0.5;
        } else if (clamped < 0.75) {
            const t = (clamped - 0.6) / 0.15;
            r = 0.5 + t * 0.3; g = 1 - t * 0.3; b = 0.5;
        } else if (clamped < 0.9) {
            const t = (clamped - 0.75) / 0.15;
            r = 0.8 + t * 0.2; g = 0.7 - t * 0.4; b = 0.5 - t * 0.4;
        } else {
            const t = (clamped - 0.9) / 0.1;
            r = 1; g = 0.3 - t * 0.25; b = 0.1 - t * 0.1;
        }

        return { r, g, b };
    }

    updateBlanket(length, thickness) {
        if (this.blanketMesh) {
            this.scene.remove(this.blanketMesh);
            this.blanketMesh.geometry.dispose();
            this.blanketMesh.material.dispose();
        }

        if (!length || !thickness) return;

        const blanketGeo = new THREE.BoxGeometry(length, thickness, 14);
        const blanketMat = new THREE.MeshStandardMaterial({
            color: 0x8844aa,
            transparent: true,
            opacity: 0.7,
            roughness: 0.85
        });

        this.blanketMesh = new THREE.Mesh(blanketGeo, blanketMat);
        this.blanketMesh.position.set(length / 2, thickness / 2, 0);
        this.scene.add(this.blanketMesh);
    }

    updateSensorMarkers(configs, dataMap) {
        for (const marker of this.sensorMarkers) {
            this.scene.remove(marker);
        }
        this.sensorMarkers = [];

        if (!this.options.showSensors) return;

        for (const cfg of configs) {
            const sensorValue = dataMap[cfg.sensor_id];

            let color = 0xf39c12;
            let scale = 1;
            let pulse = false;

            if (sensorValue != null && cfg.warning_threshold != null) {
                if (cfg.danger_threshold != null && sensorValue >= cfg.danger_threshold) {
                    color = 0xe74c3c;
                    scale = 1.5;
                    pulse = true;
                } else if (sensorValue >= cfg.warning_threshold) {
                    color = 0xf39c12;
                    scale = 1.2;
                    pulse = true;
                } else {
                    color = 0x2ecc71;
                }
            }

            const sensorGroup = new THREE.Group();

            const markerGeo = new THREE.SphereGeometry(0.5 * scale, 16, 16);
            const markerMat = new THREE.MeshStandardMaterial({
                color: color,
                emissive: color,
                emissiveIntensity: pulse ? 0.5 : 0.2,
                transparent: true,
                opacity: 0.9
            });
            const marker = new THREE.Mesh(markerGeo, markerMat);
            sensorGroup.add(marker);

            const ringGeo = new THREE.RingGeometry(0.6 * scale, 0.75 * scale, 16);
            const ringMat = new THREE.MeshBasicMaterial({
                color: color,
                transparent: true,
                opacity: 0.5,
                side: THREE.DoubleSide
            });
            const ring = new THREE.Mesh(ringGeo, ringMat);
            ring.rotation.x = Math.PI / 2;
            ring.position.y = -cfg.location_z;
            sensorGroup.add(ring);

            if (sensorValue != null) {
                const canvas = document.createElement('canvas');
                canvas.width = 128;
                canvas.height = 64;
                const ctx = canvas.getContext('2d');
                ctx.fillStyle = 'rgba(0,0,0,0.8)';
                ctx.fillRect(0, 0, 128, 64);
                ctx.strokeStyle = '#' + color.toString(16).padStart(6, '0');
                ctx.lineWidth = 2;
                ctx.strokeRect(1, 1, 126, 62);
                ctx.fillStyle = '#ffffff';
                ctx.font = 'bold 11px Consolas';
                ctx.textAlign = 'center';
                ctx.fillText(cfg.sensor_id, 64, 20);
                ctx.font = 'bold 14px Consolas';
                ctx.fillStyle = '#' + color.toString(16).padStart(6, '0');
                ctx.fillText(sensorValue.toFixed(2), 64, 42);
                ctx.font = '9px sans-serif';
                ctx.fillStyle = '#aaa';
                ctx.fillText(cfg.unit || '', 64, 56);

                const texture = new THREE.CanvasTexture(canvas);
                const spriteMat = new THREE.SpriteMaterial({ map: texture, transparent: true });
                const sprite = new THREE.Sprite(spriteMat);
                sprite.scale.set(3, 1.5, 1);
                sprite.position.y = 2;
                sprite.position.x = 0.5;
                sensorGroup.add(sprite);
            }

            sensorGroup.position.set(cfg.location_x, cfg.location_y, cfg.location_z);
            sensorGroup.userData.baseY = cfg.location_y;
            sensorGroup.userData.pulse = pulse;
            sensorGroup.userData.baseScale = scale;

            this.sensorMarkers.push(sensorGroup);
            this.scene.add(sensorGroup);
        }
    }

    setupControls() {
        let isDragging = false;
        let previousMousePosition = { x: 0, y: 0 };
        let spherical = { theta: 0.7, phi: 1.1, radius: 130 };
        const target = new THREE.Vector3(this.damLength / 2, 1, 0);

        const updateCamera = () => {
            this.camera.position.x = target.x + spherical.radius * Math.sin(spherical.phi) * Math.cos(spherical.theta);
            this.camera.position.y = target.y + spherical.radius * Math.cos(spherical.phi);
            this.camera.position.z = target.z + spherical.radius * Math.sin(spherical.phi) * Math.sin(spherical.theta);
            this.camera.lookAt(target);
        };
        updateCamera();

        this.renderer.domElement.addEventListener('mousedown', (e) => {
            isDragging = true;
            previousMousePosition = { x: e.clientX, y: e.clientY };
        });

        window.addEventListener('mouseup', () => { isDragging = false; });

        window.addEventListener('mousemove', (e) => {
            if (!isDragging) return;
            const deltaX = e.clientX - previousMousePosition.x;
            const deltaY = e.clientY - previousMousePosition.y;

            spherical.theta -= deltaX * 0.005;
            spherical.phi = Math.max(0.1, Math.min(Math.PI / 2 - 0.05, spherical.phi + deltaY * 0.005));

            updateCamera();
            previousMousePosition = { x: e.clientX, y: e.clientY };
        });

        this.renderer.domElement.addEventListener('wheel', (e) => {
            e.preventDefault();
            spherical.radius = Math.max(30, Math.min(250, spherical.radius * (1 + e.deltaY * 0.001)));
            updateCamera();
        });

        let isPanning = false;
        this.renderer.domElement.addEventListener('contextmenu', (e) => { e.preventDefault(); isPanning = true; previousMousePosition = { x: e.clientX, y: e.clientY }; });
        window.addEventListener('mouseup', () => { isPanning = false; });
        window.addEventListener('mousemove', (e) => {
            if (!isPanning) return;
            const deltaX = e.clientX - previousMousePosition.x;
            const deltaY = e.clientY - previousMousePosition.y;

            const right = new THREE.Vector3();
            const up = new THREE.Vector3(0, 1, 0);
            this.camera.getWorldDirection(right);
            right.cross(up).normalize();

            target.addScaledVector(right, -deltaX * 0.1);
            target.y += deltaY * 0.1;

            updateCamera();
            previousMousePosition = { x: e.clientX, y: e.clientY };
        });
    }

    setSimulationData(simData) {
        this.simulationData = simData;

        if (simData && simData.simulation) {
            if (simData.simulation.upstream_water_level != null &&
                simData.simulation.downstream_water_level != null) {
                this.updateWaterLevels(
                    simData.simulation.upstream_water_level,
                    simData.simulation.downstream_water_level
                );
            }
        }

        this.updatePressureCloud(simData);

        if (this.blanketMesh && simData && simData.simulation && simData.simulation.parameters) {
            const params = simData.simulation.parameters;
            if (params.blanket_enabled) {
                this.updateBlanket(params.blanket_length || 0, params.blanket_thickness || 0);
            }
        }
    }

    setOptions(options) {
        Object.assign(this.options, options);

        if (this.particleSystem) {
            this.particleSystem.visible = this.options.showStreamlines;
        }

        if (options.particleCount && options.particleCount !== this.streamParticles.length) {
            this.scene.remove(this.particleSystem);
            this.buildStreamParticles();
        }

        if (this.simulationData) {
            this.updatePressureCloud(this.simulationData);
        }

        this.updateSensorMarkers(this.sensorConfigs, this.sensorDataMap);
    }

    updateSensorConfigs(configs, dataMap) {
        this.sensorConfigs = configs;
        this.sensorDataMap = dataMap || {};
        this.updateSensorMarkers(configs, dataMap || {});
    }

    onWindowResize() {
        this.width = this.container.clientWidth;
        this.height = this.container.clientHeight;

        this.camera.aspect = this.width / this.height;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(this.width, this.height);
    }

    animate() {
        requestAnimationFrame(() => this.animate());

        const time = Date.now() * 0.001;

        this.updateParticles();

        for (const marker of this.sensorMarkers) {
            if (marker.userData.pulse) {
                const scale = 1 + Math.sin(time * 4) * 0.15 * marker.userData.baseScale;
                marker.scale.set(scale, scale, scale);
            }
        }

        this.scene.traverse((obj) => {
            if (obj.userData && obj.userData.isWaterSurface) {
                const pos = obj.geometry.attributes.position;
                for (let i = 0; i < pos.count; i++) {
                    const x = pos.getX(i);
                    const z = pos.getZ(i);
                    const wave = Math.sin(x * 0.8 + time * 2) * 0.04 +
                                Math.cos(z * 0.6 + time * 1.5) * 0.03;
                    pos.setY(i, wave);
                }
                pos.needsUpdate = true;
                obj.geometry.computeVertexNormals();
            }
        });

        this.renderer.render(this.scene, this.camera);
    }
}
