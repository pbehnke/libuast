#ifndef UAST_HPP_
#define UAST_HPP_
// C++ helpers to make it easier to implement NodeIface and work with Uast context.

#include "libuast.h"
#include <map>
#include <iostream>

namespace uast {
    // Role of UAST node.
    class Role {
    private:
        int id;
    public:
        Role(int v){
            id = v;
        }
        Role(const char* name){
            id = RoleIdForName((char*)name);
        }
        int ID() { return id; }
        char* Name() {
            return RoleNameForId(id);
        }
    };

    // Iterator is a common interface implemented by all UAST iterators.
    template<class T> class Iterator {
    public:
        virtual bool next() = 0;
        virtual T node() = 0;
    };

    // Context is a common interface implemented by all UAST contexts.
    template<class T> class Context {
    public:
        virtual void CheckError() = 0;

        virtual Iterator<T>* Filter(T root, const char* query) = 0;
        virtual Iterator<T>* Iterate(T root, TreeOrder order) = 0;
    };

    // NodeCreator is an interface that creates new UAST nodes.
    template<class T> class NodeCreator {
    public:
        virtual T NewObject(size_t size) = 0;
        virtual T NewArray(size_t size) = 0;
        virtual T NewString(const char* str) = 0;
        virtual T NewInt(int64_t val) = 0;
        virtual T NewUint(uint64_t val) = 0;
        virtual T NewFloat(double val) = 0;
        virtual T NewBool(bool val) = 0;
    };

    // NodeRawInterface is the interface of the lowest abstraction level that is needed to work with UAST nodes.
    class NodeRawInterface : public NodeCreator<NodeHandle> {
    public:
        virtual NodeKind Kind(NodeHandle node) = 0;

        virtual const char* AsString(NodeHandle node) = 0;
        virtual int64_t     AsInt(NodeHandle node) = 0;
        virtual uint64_t    AsUint(NodeHandle node) = 0;
        virtual double      AsFloat(NodeHandle node) = 0;
        virtual bool        AsBool(NodeHandle node) = 0;

        virtual size_t Size(NodeHandle node) = 0;

        virtual const char* KeyAt(NodeHandle node, size_t i) = 0;
        virtual NodeHandle ValueAt(NodeHandle node, size_t i) = 0;

        virtual void SetValue(NodeHandle node, size_t i, NodeHandle val) = 0;
        virtual void SetKeyValue(NodeHandle node, const char* key, NodeHandle val) = 0;
    };

    // Node is a high-level interface for UAST nodes.
    // Template parameter should be set to a native UAST node pointer type.
    template<class T> class Node {
    public:
        virtual NodeKind Kind() = 0;

        virtual const char* AsString() = 0;
        virtual int64_t     AsInt() = 0;
        virtual uint64_t    AsUint() = 0;
        virtual double      AsFloat() = 0;
        virtual bool        AsBool() = 0;

        virtual size_t Size() = 0;

        virtual const char* KeyAt(size_t i) = 0;
        virtual T ValueAt(size_t i) = 0;

        virtual void SetValue(size_t i, T val) = 0;
        virtual void SetKeyValue(const char* key, T val) = 0;
    };

    // RawIterator is an UAST node iterator of the lowest abstraction level.
    class RawIterator : public Iterator<NodeHandle> {
    private:
        UastIterator* iter;
        NodeHandle cur;
        bool done;
    public:
        RawIterator(UastIterator* it) {
            iter = it;
            cur = 0;
            done = false;
        }
        ~RawIterator(){
            done = true;
            cur = 0;
            UastIteratorFree(iter);
            iter = NULL;
        }
        bool next() {
            if (done) return false;
            cur = UastIteratorNext(iter);
            done = cur == 0;
            return !done;
        }
        NodeHandle node() {
            return cur;
        }
    };

    // RawContext is an UAST context at the lowest abstraction level.
    // It binds to a specific node implementation represented by NodeRawInterface.
    class RawContext : public Context<NodeHandle> {
    private:
        Uast* ctx;
        UastHandle handle;
        NodeRawInterface* impl;

        static UastHandle& lastHandle() {
            static UastHandle v = 0; return v;
        }

        static NodeIface*& cppIface() {
            static NodeIface* n = NULL;
            if (n) return n;

            n = new(NodeIface);
            n->Kind = [] (const Uast* ctx, NodeHandle node) -> NodeKind {
                auto context = getContext(ctx);
                return context->impl->Kind(node);
            };
            n->AsString = [](const Uast* ctx, NodeHandle node) -> const char* {
                auto context = getContext(ctx);
                return context->impl->AsString(node);
            };
            n->AsInt = [](const Uast* ctx, NodeHandle node) -> int64_t {
                auto context = getContext(ctx);
                return context->impl->AsInt(node);
            };
            n->AsUint = [](const Uast* ctx, NodeHandle node) -> uint64_t {
                auto context = getContext(ctx);
                return context->impl->AsUint(node);
            };
            n->AsFloat = [](const Uast* ctx, NodeHandle node) -> double {
                auto context = getContext(ctx);
                return context->impl->AsFloat(node);
            };
            n->AsBool = [](const Uast* ctx, NodeHandle node) -> bool {
                auto context = getContext(ctx);
                return context->impl->AsBool(node);
            };

            n->Size = [](const Uast* ctx, NodeHandle node) -> size_t {
                auto context = getContext(ctx);
                return context->impl->Size(node);
            };

            n->KeyAt = [](const Uast* ctx, NodeHandle node, size_t sz) -> const char* {
                auto context = getContext(ctx);
                return context->impl->KeyAt(node, sz);
            };
            n->ValueAt = [](const Uast* ctx, NodeHandle node, size_t sz) -> NodeHandle {
                auto context = getContext(ctx);
                return context->impl->ValueAt(node, sz);
            };

            n->NewObject = [](const Uast* ctx, size_t size) -> NodeHandle {
                auto context = getContext(ctx);
                return context->impl->NewObject(size);
            };
            n->NewArray = [](const Uast* ctx, size_t size) -> NodeHandle {
                auto context = getContext(ctx);
                return context->impl->NewArray(size);
            };
            n->NewString = [](const Uast* ctx, const char* str) -> NodeHandle {
                auto context = getContext(ctx);
                return context->impl->NewString(str);
            };
            n->NewInt = [](const Uast* ctx, int64_t val) -> NodeHandle {
                auto context = getContext(ctx);
                return context->impl->NewInt(val);
            };
            n->NewUint = [](const Uast* ctx, uint64_t val) -> NodeHandle {
                auto context = getContext(ctx);
                return context->impl->NewUint(val);
            };
            n->NewFloat = [](const Uast* ctx, double val) -> NodeHandle {
                auto context = getContext(ctx);
                return context->impl->NewFloat(val);
            };
            n->NewBool = [](const Uast* ctx, bool val) -> NodeHandle {
                auto context = getContext(ctx);
                return context->impl->NewBool(val);
            };

            n->SetValue = [](const Uast* ctx, NodeHandle node, size_t i, NodeHandle val) {
                auto context = getContext(ctx);
                context->impl->SetValue(node, i, val);
            };
            n->SetKeyValue = [](const Uast* ctx, NodeHandle node, const char* key, NodeHandle val) {
                auto context = getContext(ctx);
                context->impl->SetKeyValue(node, key, val);
            };
            return n;
        }

        static std::map<UastHandle,RawContext*>& contexts() {
            static std::map<UastHandle,RawContext*> v; return v;
        }

        static RawContext* getContext(const Uast* ctx) {
            return contexts()[ctx->ctx];
        }
    public:
        RawContext(NodeRawInterface* iface) {
            handle = ++lastHandle();
            contexts()[handle] = this;
            impl = iface;
            ctx = UastNew(cppIface(), handle);
            // TODO: check if pointer is valid
            CheckError();
        }
        ~RawContext() {
            UastFree(ctx);
            ctx = NULL;
            contexts().erase(handle);
            handle = 0;
            impl = NULL;
        }

        void CheckError() {
            char* err = LastError(ctx);
            if (err) throw std::runtime_error(err);
        }

        Iterator<NodeHandle>* Filter(NodeHandle root, const char * query) {
            auto it = UastFilter(ctx, root, (char*)query);
            CheckError();
            return new RawIterator(it);
        }
        Iterator<NodeHandle>* Iterate(NodeHandle root, TreeOrder order) {
            auto it = UastIteratorNew(ctx, root, order);
            CheckError();
            return new RawIterator(it);
        }
    };

    // PtrIterator is an iterator that casts NodeHandle directly to pointer type T.
    template<class T> class PtrIterator : public Iterator<T> {
    private:
         Iterator<NodeHandle>* iter;
         bool owner;
    public:
        PtrIterator(Iterator<NodeHandle>* it, bool own = false) {
            iter = it;
            owner = own;
        }
        ~PtrIterator(){
            if(owner) delete(iter);
            iter = NULL;
        }
        bool next() {
            return iter->next();
        }
        T node() {
            return (T)(iter->node());
        }
    };

    template<class T> class PtrInterface;

    // PtrContext is a Context implementation that casts NodeHandle directly to pointer type T.
    template<class T> class PtrContext : public Context<T> {
    private:
        RawContext* ctx;
    public:
        PtrContext(PtrInterface<T>* iface) {
            ctx = new RawContext(iface);
        }
        ~PtrContext(){
            delete(ctx);
            ctx = NULL;
        }
        Iterator<T>* Filter(T root, const char* query) {
            auto raw = ctx->Filter((NodeHandle)(root), query);
            auto it = new PtrIterator<T>(raw, true);
            return it;
        }
        Iterator<T>* Iterate(T root, TreeOrder order) {
            auto raw = ctx->Iterate((NodeHandle)(root), order);
            auto it = new PtrIterator<T>(raw, true);
            return it;
        }
        void CheckError(){
            return ctx->CheckError();
        }
    };

    // PtrInterface is an implementation of NodeRawInterface that casts NodeHandles directly to native node pointer T.
    template<class T> class PtrInterface : public NodeRawInterface {
    private:
        NodeCreator<T>* impl;
    public:
        PtrInterface(NodeCreator<T>* creator){
            impl = creator;
        }
        ~PtrInterface(){
        }
        Context<T>* NewContext() {
            auto ctx = new PtrContext<T>(this);
            return ctx;
        }

        NodeKind Kind(NodeHandle node) {
            Node<T>* n = (T)node;
            return n->Kind();
        }

        const char* AsString(NodeHandle node) {
            Node<T>* n = (T)node;
            return n->AsString();
        }
        int64_t AsInt(NodeHandle node) {
            Node<T>* n = (T)node;
            return n->AsInt();
        }
        uint64_t AsUint(NodeHandle node) {
            Node<T>* n = (T)node;
            return n->AsUint();
        }
        double AsFloat(NodeHandle node) {
            Node<T>* n = (T)node;
            return n->AsFloat();
        }
        bool AsBool(NodeHandle node) {
            Node<T>* n = (T)node;
            return n->AsBool();
        }

        size_t Size(NodeHandle node) {
            Node<T>* n = (T)node;
            return n->Size();
        }

        const char* KeyAt(NodeHandle node, size_t i) {
            Node<T>* n = (T)node;
            return n->KeyAt(i);
        }
        NodeHandle ValueAt(NodeHandle node, size_t i) {
            Node<T>* n = (T)node;
            T val = n->ValueAt(i);
            return (NodeHandle)(val);
        }

        NodeHandle NewObject(size_t size) {
            T node = impl->NewObject(size);
            return (NodeHandle)node;
        }
        NodeHandle NewArray(size_t size) {
            T node = impl->NewArray(size);
            return (NodeHandle)node;
        }
        NodeHandle NewString(const char* str) {
            T node = impl->NewString(str);
            return (NodeHandle)node;
        }
        NodeHandle NewInt(int64_t val) {
            T node = impl->NewInt(val);
            return (NodeHandle)node;
        }
        NodeHandle NewUint(uint64_t val) {
            T node = impl->NewUint(val);
            return (NodeHandle)node;
        }
        NodeHandle NewFloat(double val) {
            T node = impl->NewFloat(val);
            return (NodeHandle)node;
        }
        NodeHandle NewBool(bool val) {
            T node = impl->NewBool(val);
            return (NodeHandle)node;
        }

        void SetValue(NodeHandle node, size_t i, NodeHandle v) {
            Node<T>* n = (T)node;
            T val = (T)v;
            n->SetValue(i, val);
        }
        void SetKeyValue(NodeHandle node, const char* key, NodeHandle v) {
            Node<T>* n = (T)node;
            T val = (T)v;
            n->SetKeyValue(key, val);
        }
    };

} // namespace uast
#endif // UAST_HPP_