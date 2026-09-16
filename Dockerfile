# Full-featured otter container
# Includes:
# - All submodule tools (enva, fastqcx, xenofilx, pairbam, seq2mat, methx, qctb, matsrun)
# - Main otter binary
# - Three runtime conda environments: otter-core, otter-snakemake, otter-extra
# - Build helper environments: go-env, rust_build
#
# Notes:
# - Runtime environment initialization uses enva (rattler-first).
# - Environment and HDF5 runtime variables are configured via ENV.

FROM continuumio/miniconda3:latest

SHELL ["/bin/bash", "-lc"]

ENV DEBIAN_FRONTEND=noninteractive
ENV OTTER_HOME=/opt/otter
ENV OTTER_BIN=/opt/otter/bin
ENV MAMBA_ROOT_PREFIX=/opt/mamba

# Deterministic prefixes for the 3 runtime envs created by enva.
ENV OTTER_CORE_PREFIX=${MAMBA_ROOT_PREFIX}/envs/otter-core
ENV OTTER_SNAKEMAKE_PREFIX=${MAMBA_ROOT_PREFIX}/envs/otter-snakemake
ENV OTTER_EXTRA_PREFIX=${MAMBA_ROOT_PREFIX}/envs/otter-extra

# HDF5 runtime configuration (for methx and related tooling).
ENV HDF5_DIR=${OTTER_CORE_PREFIX}
ENV HDF5_INCLUDE_DIR=${HDF5_DIR}/include
ENV HDF5_LIB_DIR=${HDF5_DIR}/lib
ENV LD_LIBRARY_PATH=${HDF5_LIB_DIR}:${LD_LIBRARY_PATH}
ENV PKG_CONFIG_PATH=${HDF5_LIB_DIR}/pkgconfig:${PKG_CONFIG_PATH}

# Global PATH with all runtime env bins available out-of-box.
ENV PATH=/opt/otter/bin:/root/.cargo/bin:/opt/conda/bin:${OTTER_CORE_PREFIX}/bin:${OTTER_SNAKEMAKE_PREFIX}/bin:${OTTER_EXTRA_PREFIX}/bin:${PATH}

# System packages required for building Go/Rust submodule tools.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       bash \
       ca-certificates \
       clang \
       curl \
       git \
       pkg-config \
       build-essential \
       libssl-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR ${OTTER_HOME}
COPY . ${OTTER_HOME}

# Configure conda channels.
RUN conda config --system --set channel_priority strict \
    && conda config --system --add channels conda-forge \
    && conda config --system --add channels bioconda

# Toolchain environments used by existing build scripts.
RUN conda create -y -n go-env go \
    && conda create -y -n rust_build rust hdf5

# Prepare deterministic mamba root directory.
RUN mkdir -p ${MAMBA_ROOT_PREFIX} ${OTTER_BIN}

# Build bootstrap enva binary first, then use enva for the 3 runtime envs.
RUN conda run -n rust_build cargo build --manifest-path enva/Cargo.toml --release \
    && cp enva/target/release/enva ${OTTER_BIN}/enva

# Three primary runtime environments (out-of-box), created via enva.
RUN ${OTTER_BIN}/enva create --yaml inst/envs/otter-core.yaml --name otter-core --force --clean-cache \
    && ${OTTER_BIN}/enva create --yaml inst/envs/otter-snakemake.yaml --name otter-snakemake --force \
    && ${OTTER_BIN}/enva create --yaml inst/envs/otter-extra.yaml --name otter-extra --force

# Build main binary + all submodule tools using project-provided script.
RUN conda run -n go-env go build -o ${OTTER_BIN}/otter . \
    && STRICT_MODE=1 bash scripts/build-all-submodules.sh \
    && cp /root/.cargo/bin/enva ${OTTER_BIN}/enva \
    && cp /root/.cargo/bin/fastqcx ${OTTER_BIN}/fastqcx \
    && cp /root/.cargo/bin/xenofilx ${OTTER_BIN}/xenofilx \
    && cp /root/.cargo/bin/pairbam ${OTTER_BIN}/pairbam \
    && cp /root/.cargo/bin/seq2mat ${OTTER_BIN}/seq2mat \
    && cp /root/.cargo/bin/methx ${OTTER_BIN}/methx \
    && cp /root/.cargo/bin/qctb ${OTTER_BIN}/qctb \
    && cp /root/.cargo/bin/matsrun ${OTTER_BIN}/matsrun

# Keep image ready for immediate interactive use.
WORKDIR /workspace
CMD ["bash"]
