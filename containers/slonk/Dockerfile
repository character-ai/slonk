ARG BASE_IMAGE=ubuntu:22.04

FROM $BASE_IMAGE

# must be after the FROM
# https://stackoverflow.com/a/66995685
ARG EXTRA_INSTALL=""
ARG EXTRA_PIP_INSTALL=""
ARG LD_LIBRARY_PATH=""
ENV EXTRA_INSTALL=$EXTRA_INSTALL
ENV EXTRA_PIP_INSTALL=$EXTRA_PIP_INSTALL
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH

# # unminimize the image -- reinstalls things like manpages, etc.
RUN yes | unminimize

# carve out user accounts
ENV MUNGE_USER=11101
ENV SLURM_USER=64030
RUN addgroup --gid $MUNGE_USER munge && \
    useradd munge -u $MUNGE_USER -g $MUNGE_USER -s /usr/sbin/nologin && \
    addgroup --gid $SLURM_USER slurm  && \
    useradd slurm -u $SLURM_USER -g $SLURM_USER -s /usr/sbin/nologin

# add a bunch of keys before we install packages
RUN apt-get -qq update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -yqq gpg curl lsb-core gnupg && \
    curl https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" > /etc/apt/sources.list.d/hashicorp.list && \
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg && \
    chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null && \
    apt-get install -yqq apt-transport-https ca-certificates gnupg && \
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/cloud.google.gpg add - && \
    echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee /etc/apt/sources.list.d/google-cloud-sdk.list > /dev/null && \
    mkdir -p /etc/apt/keyrings && chmod 755 /etc/apt/keyrings && \
    curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.29/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg && \
    echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /' | tee /etc/apt/sources.list.d/kubernetes.list && \
    curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | tee /usr/share/keyrings/helm.gpg > /dev/null && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | tee /etc/apt/sources.list.d/helm-stable-debian.list && \
    curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | tee /usr/share/keyrings/cloudflare-main.gpg > /dev/null && \
    echo "deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/cloudflared.list && \
    curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null && \
    echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | tee /etc/apt/sources.list.d/ngrok.list && \
    curl 'https://content.mellanox.com/ofed/RPM-GPG-KEY-Mellanox' | gpg --dearmor -o /etc/apt/keyrings/mellanox.gpg && \
    curl https://linux.mellanox.com/public/repo/mlnx_ofed/latest/ubuntu22.04/mellanox_mlnx_ofed.list > /etc/apt/sources.list.d/mellanox.list && \
    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys A024F6F0E6D6A281 && \
    apt clean


# WARNING DO NOT PUT NEW INSTALLATIONS IN THIS SECTION. PUT THEM IN THE NEXT SECTION
# AT THE VERY END!!
RUN apt-get -qq update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -yqq --allow-change-held-packages \
        locales \
        build-essential libtool autoconf automake autotools-dev unzip curl tini \
        gettext-base ca-certificates gnupg gpg libsecret-1-0 iputils-ping net-tools \
        libnuma1 libnuma-dev numactl libsubunit0 libpci-dev libreadline-dev hwloc \
        libhwloc-dev liblz4-dev lua5.3 lua5.3-dev liblua5.3-dev pkg-config \
        munge libmunge-dev libpam0g-dev libpmix-dev libpmi2-0 libpmi2-0-dev \
        realmd adcli krb5-user dialog openssh-server less environment-modules \
        libyaml-0-2 libhttp-parser2.9 libjwt0 libyaml-dev libjson-c-dev libdbus-1-dev \
        libhttp-parser-dev libjwt-dev libmysqlclient-dev xz-utils \
        python3-ldap python3-setuptools python3-pip \
        man manpages-posix screen tmux telnet git lsof zsh vim htop \
        mosh sudo apt-transport-https rsyslog software-properties-common dnsutils \
        ncdu silversearcher-ag aptitude pv mysql-server fortune lolcat figlet \
        libjemalloc-dev openmpi-bin openmpi-common libopenmpi-dev iotop iftop \
        ninja-build uuid-runtime tree parallel zstd pigz pbzip2 bc bat sysstat \
        libpam-yubico terraform gh google-cloud-cli \
        kubectl google-cloud-sdk-gke-gcloud-auth-plugin \
        helm terraform jq cloudflared ngrok ethtool iproute2 \
        dmidecode ripgrep fd-find gdb \
        fuse-overlayfs \
        fortunes-mario fortunes-off fortune-anarchism fortunes-debian-hints \
        fortunes-bofh-excuses fortunes-spam fortunes-zh

# install nsscache
ADD https://github.com/google/libnss-cache/archive/refs/tags/version/0.21.tar.gz /tmp/libnss-cache-0.21.tar.gz
ADD https://github.com/google/nsscache/archive/refs/tags/version/0.48.tar.gz /tmp/nsscache-0.48.tar.gz
COPY ./resources/nsscache.patch /tmp/nsscache.patch
RUN tar -C /tmp -xzf /tmp/libnss-cache-0.21.tar.gz && \
    cd /tmp/libnss-cache-version-0.21/ && \
    make && \
    make install && \
    cd - && \
    rm -rf /tmp/libnss-cache-version-0.21 /tmp/libnss-cache-0.21.tar.gz && \
    tar -C /tmp -xzf /tmp/nsscache-0.48.tar.gz && \
    cd /tmp/nsscache-version-0.48/ && \
    patch -p1 < ../nsscache.patch && \
    python3 setup.py install && \
    cp examples/authorized-keys-command.py /usr/sbin && \
    cd - && \
    rm -rf /tmp/nsscache-version-0.48 /tmp/nsscache-0.48.tar.gz /tmp/nsscache.patch && \
    rm /lib/libnss_cache.so /lib/libnss_cache.so.2 && \
    ln -s /lib/libnss_cache.so.2.0 /lib/libnss_cache.so && \
    ln -s /lib/libnss_cache.so.2.0 /lib/libnss_cache.so.2

# install miniconda
RUN wget https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh && \
    bash Miniconda3-latest-Linux-x86_64.sh -b -p /opt/miniconda3 && \
    rm Miniconda3-latest-Linux-x86_64.sh

# download & build slurm
ENV SLURM_COMMIT=c7b8136cf1a2cf8a56288ceddf99ce20e129246d
ENV SLURM_COMMIT_SHORT=c7b8136
ENV BUILD_PARALLELISM=8
WORKDIR /build
COPY ./resources/init-d-slurmctld /etc/init.d/slurmctld
COPY ./resources/init-d-slurmd /etc/init.d/slurmd
COPY ./resources/init-d-slurmdbd /etc/init.d/slurmdbd
COPY ./resources/init-d-slurmrestd /etc/init.d/slurmrestd
COPY ./resources/init-d-slonklet-controller /etc/init.d/slonklet-controller
RUN curl --output slurm.tar -L https://github.com/SchedMD/slurm/tarball/$SLURM_COMMIT && \
    tar xf slurm.tar && \
    rm slurm.tar && \
    cd SchedMD-slurm-$SLURM_COMMIT_SHORT && \
    ./configure \
        --prefix=/usr --sysconfdir=/etc/slurm --disable-glibtest --disable-gtktest \
        --enable-pam --without-hdf5 --without-datawarp --disable-really-no-cray \
        --disable-cray-network --with-pam_dir=/lib/x86_64-linux-gnu/security \
        --with-json=yes --with-yaml=yes --with-http-parser=yes --with-jwt=yes --with-pmix && \
    printf "Nothing:\n\nall: Nothing\n\ninstall: Nothing\n\n.SILENT:">doc/Makefile && \
    make -j $BUILD_PARALLELISM && \
    make -j $BUILD_PARALLELISM contrib && \
    make install && \
    cd contribs/pam_slurm_adopt && make && make install && \
    cd /build && \
    chmod +x /etc/init.d/slurmctld /etc/init.d/slurmd /etc/init.d/slurmdbd /etc/init.d/slurmrestd /etc/init.d/slonklet-controller

# weirdness with pdsh
RUN chown root:root /usr/lib

# prep logs
RUN mkdir -p /var/log/slurm && touch /var/log/slurm/task_prolog.log && chown -R slurm:slurm /var/log/slurm

# containerization stuff
# enroot
ENV ENROOT_VERSION=3.4.1
WORKDIR /build
# enroot
RUN arch=$(dpkg --print-architecture) && \
    curl -fSsL -O https://github.com/NVIDIA/enroot/releases/download/v${ENROOT_VERSION}/enroot-hardened_${ENROOT_VERSION}-1_${arch}.deb && \
    curl -fSsL -O https://github.com/NVIDIA/enroot/releases/download/v${ENROOT_VERSION}/enroot-hardened+caps_${ENROOT_VERSION}-1_${arch}.deb && \
    apt-get install -y ./enroot-hardened_${ENROOT_VERSION}-1_${arch}.deb ./enroot-hardened+caps_${ENROOT_VERSION}-1_${arch}.deb && \
    rm -f ./enroot-hardened_${ENROOT_VERSION}-1_${arch}.deb ./enroot-hardened+caps_${ENROOT_VERSION}-1_${arch}.deb && \
    apt-get clean

# pyxis
ENV PYXIS_VERSION=0.15.0
RUN curl -fSsL "https://github.com/NVIDIA/pyxis/archive/refs/tags/v$PYXIS_VERSION.tar.gz" \
        -o "pyxis-v${PYXIS_VERSION}.tar.gz" && \
    tar -xzf "pyxis-v${PYXIS_VERSION}.tar.gz" && \
    cd pyxis-${PYXIS_VERSION} && \
    make install && \
    rm -rf ./pyxis-v${PYXIS_VERSION}.tar.gz ./pyxis-${PYXIS_VERSION}

# nvidia-container-toolkit
RUN distribution=$(. /etc/os-release; echo $ID$VERSION_ID) && \
    curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
        gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg && \
    curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | \
        sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
        tee /etc/apt/sources.list.d/nvidia-container-toolkit.list && \
    apt update && \
    apt install -y libnvidia-container-tools nvidia-container-toolkit && \
    sed -i 's/"--ldconfig=@$(command -v ldconfig.real || command -v ldconfig)"//' /etc/enroot/hooks.d/98-nvidia.sh

# setup munge!
RUN mkdir -p /build /run/munge /var/spool
RUN chown -R munge:munge /etc/munge /var/lib/munge /var/log/munge /run/munge && \
    rm /etc/munge/munge.key
RUN pam-auth-update --enable mkhomedir
RUN mkdir -p /var/run/sshd

# configure sshd
RUN echo "ClientAliveInterval 60" >> /etc/ssh/sshd_config && \
    echo "ClientAliveCountMax 2" >> /etc/ssh/sshd_config && \
    echo "HostKey /etc/ssh/ssh_host_rsa_key" >> /etc/ssh/sshd_config && \
    echo "HostKey /etc/ssh/ssh_host_ed25519_key" >> /etc/ssh/sshd_config && \
    echo "HostKey /etc/ssh/ssh_host_ecdsa_key" >> /etc/ssh/sshd_config

# configure mysql
RUN chmod a+rx /var/run/mysqld
RUN echo "socket = /var/run/mysqld/mysqld.sock" >> /etc/mysql/mysql.conf.d/mysqld.cnf

# some ulimits that are helpful
RUN echo "* hard memlock unlimited" >> /etc/security/limits.conf
RUN echo "* hard nofile unlimited" >> /etc/security/limits.conf
RUN echo "* hard as unlimited" >> /etc/security/limits.conf
RUN echo "* hard data unlimited" >> /etc/security/limits.conf
RUN echo "* hard nproc unlimited" >> /etc/security/limits.conf

# some pip packages to install in the root
RUN pip install -U \
    pip \
    virtualenv\
    pyyaml \
    cmake \
    pybind11 \
    black==22.6.0 \
    cai-logging \
    Flask==2.1.2 \
    pandas==2.1.0 \
    Werkzeug==2.3.7 \
    terminaltexteffects \
    $EXTRA_PIP_INSTALL

# set python3 as the default python
RUN update-alternatives --install /usr/bin/python python /usr/bin/python3 10 && \
    sudo update-alternatives --config python

# FZF
ENV FZF_VERSION=0.42.0
ENV FZF_SHA=d471067e3f46f64e6401d1c5717424535fe4c96c
RUN wget https://github.com/junegunn/fzf/releases/download/$FZF_VERSION/fzf-$FZF_VERSION-linux_amd64.tar.gz && \
    tar xvzf fzf-$FZF_VERSION-linux_amd64.tar.gz && \
    mv fzf /usr/local/bin/fzf && \
    rm fzf-$FZF_VERSION-linux_amd64.tar.gz && \
    wget https://raw.githubusercontent.com/junegunn/fzf/$FZF_SHA/shell/key-bindings.zsh -O /etc/zsh/fzf-key-bindings && \
    wget https://raw.githubusercontent.com/junegunn/fzf/$FZF_SHA/shell/completion.zsh -O /etc/zsh/fzf-completion && \
    echo "source /etc/zsh/fzf-key-bindings" >> /etc/zsh/zshrc && \
    echo "source /etc/zsh/fzf-completion" >> /etc/zsh/zshrc && \
    wget https://raw.githubusercontent.com/junegunn/fzf/$FZF_SHA/shell/key-bindings.bash -O /etc/fzf-key-bindings.bash && \
    wget https://raw.githubusercontent.com/junegunn/fzf/$FZF_SHA/shell/completion.bash -O /etc/fzf-completion.bash && \
    echo "source /etc/fzf-key-bindings.bash" >> /etc/bash.bashrc && \
    echo "source /etc/fzf-completion.bash" >> /etc/bash.bashrc

# azcopy
RUN wget https://aka.ms/downloadazcopy-v10-linux && \
    tar -xvf downloadazcopy-v10-linux && \
    chown root:root ./azcopy_linux_amd64_*/azcopy && \
    chmod 755 ./azcopy_linux_amd64_*/azcopy && \
    mv  ./azcopy_linux_amd64_*/azcopy /usr/local/bin/azcopy && \
    rm -f downloadazcopy-v10-linux && \
    rm -rf ./azcopy_linux_amd64_*

# argocd
RUN curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64 && \
    install -m 555 argocd-linux-amd64 /usr/local/bin/argocd && \
    rm argocd-linux-amd64

RUN cd "$(mktemp -d)" && \
    OS="$(uname | tr '[:upper:]' '[:lower:]')" && \
    ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" && \
    KREW="krew-${OS}_${ARCH}" && \
    curl -fsSLO "https://github.com/kubernetes-sigs/krew/releases/latest/download/${KREW}.tar.gz" && \
    tar zxvf "${KREW}.tar.gz" && \
    cp ./"${KREW}" /usr/local/bin/krew && \
    rm -rf "${KREW}.tar.gz"

# custom pdsh
RUN cd "/build" && \
    git clone https://github.com/chaos/pdsh.git pdsh && \
    cd pdsh && \
    ./bootstrap && \
    ./configure --with-fanout=64 --with-ssh --without-rsh --with-slurm --with-nodeupdown \
        --with-readline --enable-static-modules && \
    make && \
    make install && \
    cd .. && \
    rm -rf pdsh

# kubecolor
RUN wget https://github.com/hidetatz/kubecolor/releases/download/v0.0.25/kubecolor_0.0.25_Linux_x86_64.tar.gz && \
    tar xvzf kubecolor_0.0.25_Linux_x86_64.tar.gz && \
    cp kubecolor /usr/local/bin/kubecolor && \
    mv kubecolor /usr/local/bin/k && \
    rm kubecolor_0.0.25_Linux_x86_64.tar.gz

# install go
RUN cd /build && \
    wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz && \
    rm go1.22.0.linux-amd64.tar.gz && \
    echo 'export PATH="/usr/local/go/bin:$PATH"' > /etc/profile.d/golang.sh

RUN . /etc/profile.d/golang.sh && \
     git clone https://github.com/grafana/loki.git && \
     cd loki && \
     make logcli && \
     cp cmd/logcli/logcli /usr/local/bin/logcli

# Last second make sure all the libraries are found
ENV LD_LIBRARY_PATH=$LD_LIBRARY_PATH
RUN sudo /sbin/ldconfig

# more figlet
RUN cd /build && \
    git clone https://github.com/hIMEI29A/FigletFonts.git figlet && \
    cd figlet && \
    make && \
    cd /build && \
    rm -rf figlet

# 24 hour timestamps
RUN locale-gen en_US.UTF-8 && \
    update-locale LANG=en_US.UTF-8
ENV LC_TIME="C.UTF-8"

# bat
RUN ln -s /usr/bin/batcat /usr/bin/bat

# MAKE SURE TO LEAVE EXTRA_INSTALL, WHICH COVERS SLONK-H100
RUN add-apt-repository ppa:jgmath2000/et && \
    apt-add-repository ppa:ubuntuhandbook1/emacs && \
    wget -qO - 'https://proget.makedeb.org/debian-feeds/prebuilt-mpr.pub' | gpg --dearmor | tee /usr/share/keyrings/prebuilt-mpr-archive-keyring.gpg 1> /dev/null && \
    echo "deb [arch=all,$(dpkg --print-architecture) signed-by=/usr/share/keyrings/prebuilt-mpr-archive-keyring.gpg] https://proget.makedeb.org prebuilt-mpr $(lsb_release -cs)" | tee /etc/apt/sources.list.d/prebuilt-mpr.list && \
    apt-get -qq update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -yqq --allow-downgrades --allow-change-held-packages \
        buildah et emacs strace neovim \
        nginx \
        nethack-console \
        libpam-yubico \
        nfs-common \
        just \
        ${EXTRA_INSTALL} && \
    apt-get clean

COPY extra.sh /build
RUN bash /build/extra.sh


