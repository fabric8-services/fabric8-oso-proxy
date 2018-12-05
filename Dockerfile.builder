FROM centos:7
MAINTAINER "Aslak Knutsen <aslak@redhat.com>"
ENV LANG=en_US.utf8

# Some packages might seem weird but they are required by the RVM installer.
RUN yum install epel-release --enablerepo=extras -y \
    && yum --enablerepo=centosplus --enablerepo=epel-testing install -y \
      findutils \
      git \
      $(test "$USE_GO_VERSION_FROM_WEBSITE" != 1 && echo "golang") \
      make \
      mercurial \
      procps-ng \
      tar \
      wget \
      which \
      gcc \
    && yum clean all \
    && rm -rf /var/cache/yum

RUN if [[ "$USE_GO_VERSION_FROM_WEBSITE" = 1 ]]; then cd /tmp \
    && wget https://storage.googleapis.com/golang/go1.10.4.linux-amd64.tar.gz  \
    && echo "fa04efdb17a275a0c6e137f969a1c4eb878939e91e1da16060ce42f02c2ec5ec go1.10.4.linux-amd64.tar.gz" > checksum \
    && sha256sum -c checksum \
    && tar -C /usr/local -xzf go1.10.4.linux-amd64.tar.gz \
    && rm -f go1.10.4.linux-amd64.tar.gz; \
    fi
ENV GOPATH=/tmp/go
RUN mkdir -p ${GOPATH}/bin
RUN chmod -R a+rwx ${GOPATH}
ENV PATH=$PATH:/usr/local/go/bin:${GOPATH}/bin

ENTRYPOINT ["/bin/bash"]
